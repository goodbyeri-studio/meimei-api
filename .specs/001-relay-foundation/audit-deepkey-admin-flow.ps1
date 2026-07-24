param(
    [string]$RelayBaseUrl = "http://127.0.0.1:3010",
    [string]$PostgresContainer = "blackrain-relay-dev-pg",
    [int]$ChannelId = 10,
    [string]$Model = "gpt-5.4",
    [string]$OutputPath = ""
)

$ErrorActionPreference = "Stop"
$RelayBaseUrl = $RelayBaseUrl.TrimEnd("/")
$auditGroup = "audit-deepkey-manual"

function Get-PostgresConnection {
    $inspect = docker inspect $PostgresContainer | ConvertFrom-Json
    if (-not $inspect) {
        throw "PostgreSQL container not found: $PostgresContainer"
    }
    $envMap = @{}
    foreach ($entry in $inspect[0].Config.Env) {
        $parts = $entry -split "=", 2
        $envMap[$parts[0]] = $parts[1]
    }
    if (-not $envMap["POSTGRES_USER"] -or -not $envMap["POSTGRES_DB"]) {
        throw "PostgreSQL container configuration is incomplete"
    }
    return @{
        User = $envMap["POSTGRES_USER"]
        Database = $envMap["POSTGRES_DB"]
    }
}

$postgres = Get-PostgresConnection

function Invoke-Psql([string]$Sql) {
    $output = @(docker exec $PostgresContainer psql -v ON_ERROR_STOP=1 -U $postgres.User -d $postgres.Database -F "|" -Atc $Sql)
    if ($LASTEXITCODE -ne 0) {
        throw "PostgreSQL command failed"
    }
    return $output
}

function Set-SystemOption(
    [Microsoft.PowerShell.Commands.WebRequestSession]$Session,
    [hashtable]$Headers,
    [string]$Key,
    [string]$Value
) {
    $response = Invoke-RestMethod -Method Put -Uri "$RelayBaseUrl/api/option/" -WebSession $Session -Headers $Headers -ContentType "application/json" -Body (@{
        key = $Key
        value = $Value
    } | ConvertTo-Json -Compress) -TimeoutSec 30
    if (-not $response.success) {
        throw "Updating option $Key failed: $($response.message)"
    }
}

function Set-ChannelStatus(
    [Microsoft.PowerShell.Commands.WebRequestSession]$Session,
    [hashtable]$Headers,
    [int]$Id,
    [int]$Status
) {
    $response = Invoke-RestMethod -Method Post -Uri "$RelayBaseUrl/api/channel/$Id/status" -WebSession $Session -Headers $Headers -ContentType "application/json" -Body (@{
        status = $Status
    } | ConvertTo-Json -Compress) -TimeoutSec 30
    if (-not $response.success) {
        throw "Updating channel $Id status failed: $($response.message)"
    }
}

function Invoke-ModelCall([string]$TokenKey) {
    $payload = @{
        model = $Model
        messages = @(@{
            role = "user"
            content = "Reply with exactly OK."
        })
        max_tokens = 2
        temperature = 0
        stream = $false
    } | ConvertTo-Json -Depth 5
    $response = Invoke-WebRequest -Method Post -Uri "$RelayBaseUrl/v1/chat/completions" -Headers @{
        Authorization = "Bearer sk-$TokenKey"
    } -ContentType "application/json" -Body $payload -SkipHttpErrorCheck -TimeoutSec 180
    return [int]$response.StatusCode
}

$suffix = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds().ToString().Substring(3) + [Random]::Shared.Next(100, 999)
$rootUsername = "rootaudit$suffix"
$customerUsername = "groupaudit$suffix"
$rootPassword = "Aa9!" + [Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(6)).ToLowerInvariant()
$customerPassword = "Bb8!" + [Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(6)).ToLowerInvariant()
$rootId = 0
$customerId = 0
$tokenId = 0
$rootSession = $null
$rootHeaders = $null
$originalGroupRatio = $null
$originalUserUsableGroups = $null
$originalChannelGroup = $null
$originalChannelStatus = 0
$channelRemapped = $false

try {
    $channelRow = @(Invoke-Psql "SELECT id, `"group`", status FROM channels WHERE id = $ChannelId;")
    if ($channelRow.Count -ne 1) {
        throw "Expected exactly one channel with id $ChannelId"
    }
    $channelParts = $channelRow[0] -split "\|", 3
    $originalChannelGroup = $channelParts[1]
    $originalChannelStatus = [int]$channelParts[2]
    if ($originalChannelStatus -ne 1) {
        throw "Channel $ChannelId must be enabled before the audit"
    }
    $originalGroupRatio = [string](@(Invoke-Psql "SELECT value FROM options WHERE key = 'GroupRatio';")[0])
    $originalUserUsableGroups = [string](@(Invoke-Psql "SELECT value FROM options WHERE key = 'UserUsableGroups';")[0])
    if (-not $originalGroupRatio -or -not $originalUserUsableGroups) {
        throw "Group option snapshot is incomplete"
    }

    foreach ($registration in @(
        @{ username = $rootUsername; password = $rootPassword },
        @{ username = $customerUsername; password = $customerPassword }
    )) {
        $response = Invoke-RestMethod -Method Post -Uri "$RelayBaseUrl/api/user/register?turnstile=" -ContentType "application/json" -Body ($registration | ConvertTo-Json -Compress) -TimeoutSec 30
        if (-not $response.success) {
            throw "Audit user registration failed: $($response.message)"
        }
    }

    $userRows = Invoke-Psql "SELECT username, id FROM users WHERE username IN ('$rootUsername','$customerUsername');"
    foreach ($line in $userRows) {
        $parts = $line -split "\|", 2
        if ($parts[0] -eq $rootUsername) {
            $rootId = [int]$parts[1]
        } elseif ($parts[0] -eq $customerUsername) {
            $customerId = [int]$parts[1]
        }
    }
    if ($rootId -le 0 -or $customerId -le 0) {
        throw "Audit user IDs are missing"
    }
    Invoke-Psql "UPDATE users SET role = 100, status = 1 WHERE id = $rootId; UPDATE users SET quota = 1000000000 WHERE id = $customerId;" | Out-Null

    $rootSession = New-Object Microsoft.PowerShell.Commands.WebRequestSession
    $rootLogin = Invoke-RestMethod -Method Post -Uri "$RelayBaseUrl/api/user/login?turnstile=" -WebSession $rootSession -ContentType "application/json" -Body (@{
        username = $rootUsername
        password = $rootPassword
    } | ConvertTo-Json -Compress) -TimeoutSec 30
    if (-not $rootLogin.success -or [int]$rootLogin.data.role -ne 100) {
        throw "Temporary root login failed"
    }
    $rootHeaders = @{ "New-Api-User" = "$rootId" }

    $customerSession = New-Object Microsoft.PowerShell.Commands.WebRequestSession
    $customerLogin = Invoke-RestMethod -Method Post -Uri "$RelayBaseUrl/api/user/login?turnstile=" -WebSession $customerSession -ContentType "application/json" -Body (@{
        username = $customerUsername
        password = $customerPassword
    } | ConvertTo-Json -Compress) -TimeoutSec 30
    if (-not $customerLogin.success -or [int]$customerLogin.data.id -ne $customerId) {
        throw "Temporary customer login failed"
    }
    $customerHeaders = @{ "New-Api-User" = "$customerId" }

    Set-ChannelStatus $rootSession $rootHeaders $ChannelId 2
    Invoke-Psql "UPDATE channels SET `"group`" = '$auditGroup' WHERE id = $ChannelId;" | Out-Null
    $channelRemapped = $true
    Set-ChannelStatus $rootSession $rootHeaders $ChannelId 1

    $groupRatio = $originalGroupRatio | ConvertFrom-Json -AsHashtable
    $userUsableGroups = $originalUserUsableGroups | ConvertFrom-Json -AsHashtable
    $groupRatio[$auditGroup] = 2.0
    $userUsableGroups[$auditGroup] = "DeepKey production audit group"
    Set-SystemOption $rootSession $rootHeaders "GroupRatio" ($groupRatio | ConvertTo-Json -Compress)
    Set-SystemOption $rootSession $rootHeaders "UserUsableGroups" ($userUsableGroups | ConvertTo-Json -Compress)

    $tokenResponse = Invoke-RestMethod -Method Post -Uri "$RelayBaseUrl/api/token/" -WebSession $customerSession -Headers $customerHeaders -ContentType "application/json" -Body (@{
        name = "audit-manual-group"
        expired_time = -1
        remain_quota = 0
        unlimited_quota = $true
        model_limits_enabled = $false
        model_limits = ""
        allow_ips = ""
        group = $auditGroup
        cross_group_retry = $false
    } | ConvertTo-Json -Compress) -TimeoutSec 30
    if (-not $tokenResponse.success) {
        throw "Creating a token in the added group failed: $($tokenResponse.message)"
    }

    $tokenRow = @(Invoke-Psql "SELECT id, `"key`" FROM tokens WHERE user_id = $customerId AND `"group`" = '$auditGroup' ORDER BY id DESC LIMIT 1;")
    if ($tokenRow.Count -ne 1) {
        throw "The generated customer token is missing"
    }
    $tokenParts = $tokenRow[0] -split "\|", 2
    $tokenId = [int]$tokenParts[0]
    $tokenKey = $tokenParts[1]
    if ($tokenId -le 0 -or -not $tokenKey) {
        throw "The generated customer token is incomplete"
    }

    $highRatioStatus = Invoke-ModelCall $tokenKey
    if ($highRatioStatus -ne 200) {
        throw "The 2.0 ratio model call failed with HTTP $highRatioStatus"
    }

    $groupRatio[$auditGroup] = 0.5
    Set-SystemOption $rootSession $rootHeaders "GroupRatio" ($groupRatio | ConvertTo-Json -Compress)
    $lowRatioStatus = Invoke-ModelCall $tokenKey
    if ($lowRatioStatus -ne 200) {
        throw "The 0.5 ratio model call failed with HTTP $lowRatioStatus"
    }

    $logRows = Invoke-Psql @"
SELECT id, user_id, token_id, channel_id, prompt_tokens, completion_tokens, quota, "group"
FROM logs
WHERE type = 2 AND user_id = $customerId AND token_id = $tokenId
ORDER BY id;
"@
    $consumeLogs = @()
    foreach ($line in $logRows) {
        $parts = $line -split "\|", 8
        $consumeLogs += [pscustomobject]@{
            Id = [int]$parts[0]
            UserId = [int]$parts[1]
            TokenId = [int]$parts[2]
            ChannelId = [int]$parts[3]
            PromptTokens = [int]$parts[4]
            CompletionTokens = [int]$parts[5]
            Quota = [int]$parts[6]
            Group = $parts[7]
        }
    }
    if ($consumeLogs.Count -ne 2) {
        throw "Expected two consume logs, got $($consumeLogs.Count)"
    }
    foreach ($log in $consumeLogs) {
        if ($log.UserId -ne $customerId -or $log.TokenId -ne $tokenId -or $log.ChannelId -ne $ChannelId -or $log.Group -ne $auditGroup -or $log.Quota -le 0) {
            throw "Consume log ownership or billing fields are incorrect"
        }
    }
    if ($consumeLogs[0].PromptTokens -ne $consumeLogs[1].PromptTokens -or $consumeLogs[0].CompletionTokens -ne $consumeLogs[1].CompletionTokens) {
        throw "The two ratio calls returned different token usage and cannot be compared"
    }
    $quotaDelta = [Math]::Abs($consumeLogs[0].Quota - (4 * $consumeLogs[1].Quota))
    if ($quotaDelta -gt 3) {
        throw "The 2.0 and 0.5 group ratios did not produce the expected fourfold charge"
    }

    $groupRatio.Remove($auditGroup)
    $userUsableGroups.Remove($auditGroup)
    Set-SystemOption $rootSession $rootHeaders "GroupRatio" ($groupRatio | ConvertTo-Json -Compress)
    Set-SystemOption $rootSession $rootHeaders "UserUsableGroups" ($userUsableGroups | ConvertTo-Json -Compress)

    $newTokenResponse = Invoke-RestMethod -Method Post -Uri "$RelayBaseUrl/api/token/" -WebSession $customerSession -Headers $customerHeaders -ContentType "application/json" -Body (@{
        name = "audit-deleted-group"
        expired_time = -1
        remain_quota = 0
        unlimited_quota = $true
        model_limits_enabled = $false
        model_limits = ""
        allow_ips = ""
        group = $auditGroup
        cross_group_retry = $false
    } | ConvertTo-Json -Compress) -TimeoutSec 30
    if ($newTokenResponse.success) {
        throw "Creating a token in the deleted group unexpectedly succeeded"
    }

    $logCountBeforeRejectedCall = [int](@(Invoke-Psql "SELECT COUNT(*) FROM logs WHERE type = 2 AND user_id = $customerId AND token_id = $tokenId;")[0])
    $oldTokenStatus = Invoke-ModelCall $tokenKey
    $logCountAfterRejectedCall = [int](@(Invoke-Psql "SELECT COUNT(*) FROM logs WHERE type = 2 AND user_id = $customerId AND token_id = $tokenId;")[0])
    if ($oldTokenStatus -eq 200 -or $logCountAfterRejectedCall -ne $logCountBeforeRejectedCall) {
        throw "The deleted group's old token was accepted or generated a consume log"
    }

    $result = [ordered]@{
        schema_version = 1
        generated_at_utc = (Get-Date).ToUniversalTime().ToString("o")
        group_added = $true
        generated_customer_token = $true
        high_ratio = 2.0
        low_ratio = 0.5
        high_ratio_http_status = $highRatioStatus
        low_ratio_http_status = $lowRatioStatus
        prompt_tokens = $consumeLogs[0].PromptTokens
        completion_tokens = $consumeLogs[0].CompletionTokens
        high_ratio_quota = $consumeLogs[0].Quota
        low_ratio_quota = $consumeLogs[1].Quota
        quota_fourfold_with_rounding = ($quotaDelta -le 3)
        user_id_preserved = (@($consumeLogs.UserId | Sort-Object -Unique).Count -eq 1)
        token_id_preserved = (@($consumeLogs.TokenId | Sort-Object -Unique).Count -eq 1)
        channel_id_preserved = (@($consumeLogs.ChannelId | Sort-Object -Unique).Count -eq 1)
        group_deleted = $true
        new_token_after_delete_rejected = (-not $newTokenResponse.success)
        old_token_after_delete_http_status = $oldTokenStatus
        rejected_call_created_consume_log = ($logCountAfterRejectedCall -ne $logCountBeforeRejectedCall)
    }
    $json = $result | ConvertTo-Json -Depth 5
    if ($OutputPath) {
        $resolvedOutput = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputPath)
        $outputDirectory = Split-Path -Parent $resolvedOutput
        if ($outputDirectory) {
            New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null
        }
        [System.IO.File]::WriteAllText($resolvedOutput, $json + [Environment]::NewLine, [System.Text.UTF8Encoding]::new($false))
        Write-Output "audit_result=$resolvedOutput"
    }
    Write-Output $json
} finally {
    if ($rootSession -and $rootHeaders) {
        if ($originalGroupRatio) {
            try { Set-SystemOption $rootSession $rootHeaders "GroupRatio" $originalGroupRatio } catch { Write-Warning $_.Exception.Message }
        }
        if ($originalUserUsableGroups) {
            try { Set-SystemOption $rootSession $rootHeaders "UserUsableGroups" $originalUserUsableGroups } catch { Write-Warning $_.Exception.Message }
        }
        if ($channelRemapped) {
            try {
                Set-ChannelStatus $rootSession $rootHeaders $ChannelId 2
                Invoke-Psql "UPDATE channels SET `"group`" = '$originalChannelGroup' WHERE id = $ChannelId;" | Out-Null
                Set-ChannelStatus $rootSession $rootHeaders $ChannelId $originalChannelStatus
                $channelRemapped = $false
            } catch {
                Write-Warning $_.Exception.Message
            }
        }
    }
    if ($channelRemapped -and $originalChannelGroup) {
        Invoke-Psql "UPDATE channels SET `"group`" = '$originalChannelGroup', status = $originalChannelStatus WHERE id = $ChannelId;" | Out-Null
    }
    if ($rootId -gt 0 -or $customerId -gt 0) {
        $ids = @($rootId, $customerId | Where-Object { $_ -gt 0 }) -join ","
        if ($ids) {
            Invoke-Psql @"
DELETE FROM logs WHERE user_id IN ($ids);
DELETE FROM quota_data WHERE user_id IN ($ids);
DELETE FROM tokens WHERE user_id IN ($ids);
DELETE FROM users WHERE id IN ($ids);
"@ | Out-Null
        }
    }
}
