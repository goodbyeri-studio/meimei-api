package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeGroupRatioOptions(t *testing.T) {
	values, err := normalizeGroupRatioOptions(GroupRatioOptionsRequest{
		GroupRatio:              `{"gpt":1.3}`,
		TopupGroupRatio:         `{"gpt":1.1}`,
		UserUsableGroups:        `{"gpt":"GPT"}`,
		GroupGroupRatio:         `{"vip":{"gpt":0.9}}`,
		AutoGroups:              `["gpt"]`,
		DefaultUseAutoGroup:     true,
		GroupSpecialUsableGroup: `{"vip":{"+:gpt":"GPT"}}`,
	})
	require.NoError(t, err)
	require.Equal(t, `{"gpt":1.3}`, values["GroupRatio"])
	require.Equal(t, "true", values["DefaultUseAutoGroup"])
	require.Equal(t, `{"vip":{"gpt":0.9}}`, values["GroupGroupRatio"])
}

func TestNormalizeGroupRatioOptionsRejectsInvalidRatios(t *testing.T) {
	_, err := normalizeGroupRatioOptions(GroupRatioOptionsRequest{
		GroupRatio:      `{"gpt":1}`,
		GroupGroupRatio: `{"vip":{"gpt":-0.1}}`,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "GroupGroupRatio")
}

func TestNormalizeGroupRatioOptionsRejectsNullCollections(t *testing.T) {
	_, err := normalizeGroupRatioOptions(GroupRatioOptionsRequest{
		GroupRatio:       `null`,
		TopupGroupRatio:  `{}`,
		UserUsableGroups: `{}`,
		GroupGroupRatio:  `{}`,
		AutoGroups:       `[]`,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "GroupRatio")
}

func TestValidateGroupRemovalSafetyRejectsGroupsUsedByCustomerTokens(t *testing.T) {
	err := validateGroupRemovalSafety(
		map[string]float64{"kept-group": 1, "in-use-group": 1},
		map[string]float64{"kept-group": 1},
		func(groups []string) (map[string]int64, error) {
			require.ElementsMatch(t, []string{"in-use-group"}, groups)
			return map[string]int64{"in-use-group": 2}, nil
		},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "in-use-group")

	require.NoError(t, validateGroupRemovalSafety(
		map[string]float64{"kept-group": 1, "unused-group": 1},
		map[string]float64{"kept-group": 1},
		func([]string) (map[string]int64, error) { return map[string]int64{}, nil },
	))
}
