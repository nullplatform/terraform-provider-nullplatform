package nullplatform

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// grantMap builds the map SDKv2 hands a set element: every attribute in the
// schema is present, zero-valued when the config omits it. Tests that build the
// map by hand miss exactly the bugs that live in telling "unset" from "empty".
func grantMap(overrides map[string]any) map[string]any {
	grant := map[string]any{
		"nrn":               "",
		"role_id":           0,
		"role_slug":         "",
		"actions":           schema.NewSet(schema.HashString, nil),
		"inherits":          schema.NewSet(schema.HashString, nil),
		"add_actions":       schema.NewSet(schema.HashString, nil),
		"remove_actions":    schema.NewSet(schema.HashString, nil),
		"effective_actions": schema.NewSet(schema.HashString, nil),
	}
	for key, value := range overrides {
		if strs, ok := value.([]string); ok {
			items := make([]any, len(strs))
			for i, s := range strs {
				items[i] = s
			}
			grant[key] = schema.NewSet(schema.HashString, items)
			continue
		}
		grant[key] = value
	}
	return grant
}

func TestConvertToGrantRoleForms(t *testing.T) {
	t.Run("role_slug alone", func(t *testing.T) {
		grant := convertToGrant(grantMap(map[string]any{
			"nrn":       "organization=1:account=1",
			"role_slug": "ops",
		}))

		require.NotNil(t, grant.RoleSlug)
		assert.Equal(t, "ops", *grant.RoleSlug)
		assert.Nil(t, grant.RoleID, "an unset role_id must not travel as 0")
		assert.Nil(t, grant.Actions)
		assert.Nil(t, grant.Inherits)
	})

	// The provider used to null out RoleID for every grant whenever any grant
	// carried a role_slug key — which, since SDKv2 always populates the key,
	// was always. A role_id-only grant went out as {"role_slug": ""}.
	t.Run("role_id alone", func(t *testing.T) {
		grant := convertToGrant(grantMap(map[string]any{
			"nrn":     "organization=1:account=1",
			"role_id": 560594970,
		}))

		require.NotNil(t, grant.RoleID)
		assert.Equal(t, int64(560594970), *grant.RoleID)
		assert.Nil(t, grant.RoleSlug, "an unset role_slug must not travel as an empty string")
	})
}

func TestConvertToGrantFineGrainedForm(t *testing.T) {
	grant := convertToGrant(grantMap(map[string]any{
		"nrn":     "organization=1:account=1:namespace=1",
		"actions": []string{"deployment:create", "application:read"},
	}))

	assert.Nil(t, grant.RoleID)
	assert.Nil(t, grant.RoleSlug)
	assert.Nil(t, grant.Inherits)

	// A literal list, not the {add, remove} object the inherited form sends.
	assert.Equal(t, []string{"application:read", "deployment:create"}, grant.Actions,
		"actions must be sorted so an unordered set produces a stable request")
}

func TestConvertToGrantInheritedForm(t *testing.T) {
	t.Run("with both deltas", func(t *testing.T) {
		grant := convertToGrant(grantMap(map[string]any{
			"nrn":            "organization=1:account=1",
			"inherits":       []string{"ops", "developer"},
			"add_actions":    []string{"application:delete"},
			"remove_actions": []string{"deployment:create"},
		}))

		assert.Equal(t, []string{"developer", "ops"}, grant.Inherits)
		assert.Equal(t, map[string][]string{
			"add":    {"application:delete"},
			"remove": {"deployment:create"},
		}, grant.Actions)
	})

	// The API's schema allows `actions` to be absent entirely; sending empty
	// arrays would be a needless difference from what the user wrote.
	t.Run("without deltas", func(t *testing.T) {
		grant := convertToGrant(grantMap(map[string]any{
			"nrn":      "organization=1:account=1",
			"inherits": []string{"ops"},
		}))

		assert.Equal(t, []string{"ops"}, grant.Inherits)
		assert.Nil(t, grant.Actions, "no delta means no actions key at all")
	})

	t.Run("with only a removal", func(t *testing.T) {
		grant := convertToGrant(grantMap(map[string]any{
			"nrn":            "organization=1:account=1",
			"inherits":       []string{"ops"},
			"remove_actions": []string{"deployment:create"},
		}))

		assert.Equal(t, map[string][]string{"remove": {"deployment:create"}}, grant.Actions)
	})
}

func TestConvertFromGrantKeepsConfigShape(t *testing.T) {
	roleID := int64(9001)
	slug := "namespace:admin"

	t.Run("an existing role by slug", func(t *testing.T) {
		raw := convertFromGrant(ApiKeyGrantRead{
			NRN: "organization=1:account=1", RoleID: &roleID, RoleSlug: &slug,
		}, true)

		assert.Equal(t, "namespace:admin", raw["role_slug"])
		assert.NotContains(t, raw, "role_id",
			"writing back both would read as two shapes on the next plan")
	})

	t.Run("an existing role by id", func(t *testing.T) {
		raw := convertFromGrant(ApiKeyGrantRead{
			NRN: "organization=1:account=1", RoleID: &roleID, RoleSlug: &slug,
		}, false)

		assert.Equal(t, int64(9001), raw["role_id"])
		assert.NotContains(t, raw, "role_slug")
	})

	// The role behind a fine-grained grant is private to the key and recreated
	// whenever the grants are replaced. Writing it to state guarantees drift.
	t.Run("a fine-grained grant", func(t *testing.T) {
		raw := convertFromGrant(ApiKeyGrantRead{
			NRN:      "organization=1:account=1:namespace=1",
			RoleID:   &roleID,
			RoleSlug: &slug,
			Actions:  []string{"application:read", "deployment:create"},
		}, true)

		assert.Equal(t, []string{"application:read", "deployment:create"}, raw["actions"])
		assert.Equal(t, []string{"application:read", "deployment:create"}, raw["effective_actions"])
		assert.NotContains(t, raw, "role_id", "the key-private role must stay out of state")
		assert.NotContains(t, raw, "role_slug", "the key-private role must stay out of state")
	})

	// Written as slugs plus a delta, read back as objects plus added/removed.
	t.Run("an inherited grant", func(t *testing.T) {
		raw := convertFromGrant(ApiKeyGrantRead{
			NRN:      "organization=1:account=1",
			RoleID:   &roleID,
			RoleSlug: &slug,
			Actions:  []string{"application:read", "application:delete"},
			Inherits: []ApiKeyInheritedRole{
				{ID: 11, Slug: "ops"},
				{ID: 12, Slug: "developer"},
			},
			Added:   []string{"application:delete"},
			Removed: []string{"deployment:create"},
		}, true)

		assert.Equal(t, []string{"ops", "developer"}, raw["inherits"],
			"inherits must come back as the slugs the config was written with")
		assert.Equal(t, []string{"application:delete"}, raw["add_actions"])
		assert.Equal(t, []string{"deployment:create"}, raw["remove_actions"])
		assert.Equal(t, []string{"application:read", "application:delete"}, raw["effective_actions"])
		assert.NotContains(t, raw, "actions",
			"the inherited form's own actions live in add_actions/remove_actions")
		assert.NotContains(t, raw, "role_id")
	})
}

// grants is a TypeSet, so an element's identity is its hash. Left to the
// default, that hash covers the computed extras the API sends back and every
// refresh reports a change that is not one.
func TestApiKeyGrantHashIgnoresComputedFields(t *testing.T) {
	t.Run("a fine-grained grant", func(t *testing.T) {
		config := grantMap(map[string]any{
			"nrn":     "organization=1:account=1",
			"actions": []string{"application:read"},
		})
		refreshed := grantMap(map[string]any{
			"nrn":               "organization=1:account=1",
			"actions":           []string{"application:read"},
			"role_id":           9001,
			"role_slug":         "apikey:12333:0",
			"effective_actions": []string{"application:read"},
		})

		assert.Equal(t, apiKeyGrantHash(config), apiKeyGrantHash(refreshed))
	})

	t.Run("an existing role by slug", func(t *testing.T) {
		config := grantMap(map[string]any{
			"nrn":       "organization=1:account=1",
			"role_slug": "ops",
		})
		refreshed := grantMap(map[string]any{
			"nrn":       "organization=1:account=1",
			"role_slug": "ops",
			"role_id":   560594970,
		})

		assert.Equal(t, apiKeyGrantHash(config), apiKeyGrantHash(refreshed))
	})
}

func TestApiKeyGrantHashSeparatesDistinctGrants(t *testing.T) {
	base := grantMap(map[string]any{
		"nrn":     "organization=1:account=1",
		"actions": []string{"application:read"},
	})

	for name, other := range map[string]map[string]any{
		"another nrn": grantMap(map[string]any{
			"nrn": "organization=1:account=2", "actions": []string{"application:read"},
		}),
		"another action": grantMap(map[string]any{
			"nrn": "organization=1:account=1", "actions": []string{"application:delete"},
		}),
		"an extra action": grantMap(map[string]any{
			"nrn": "organization=1:account=1", "actions": []string{"application:read", "application:delete"},
		}),
		"the same actions, inherited": grantMap(map[string]any{
			"nrn": "organization=1:account=1", "inherits": []string{"application:read"},
		}),
	} {
		t.Run(name, func(t *testing.T) {
			assert.NotEqual(t, apiKeyGrantHash(base), apiKeyGrantHash(other))
		})
	}

	// Set members are unordered; the same two actions written the other way
	// round are the same grant.
	t.Run("the same actions in another order", func(t *testing.T) {
		one := grantMap(map[string]any{
			"nrn": "organization=1:account=1", "actions": []string{"a:read", "b:write"},
		})
		two := grantMap(map[string]any{
			"nrn": "organization=1:account=1", "actions": []string{"b:write", "a:read"},
		})
		assert.Equal(t, apiKeyGrantHash(one), apiKeyGrantHash(two))
	})
}

func TestValidateGrantShapes(t *testing.T) {
	valid := map[string][]map[string]any{
		"a role by slug": {
			grantMap(map[string]any{"nrn": "organization=1", "role_slug": "ops"}),
		},
		"a role by id": {
			grantMap(map[string]any{"nrn": "organization=1", "role_id": 42}),
		},
		"several roles on one nrn": {
			grantMap(map[string]any{"nrn": "organization=1", "role_slug": "ops"}),
			grantMap(map[string]any{"nrn": "organization=1", "role_slug": "admin"}),
		},
		"a slug next to a fine-grained grant": {
			grantMap(map[string]any{"nrn": "organization=1", "role_slug": "ops"}),
			grantMap(map[string]any{"nrn": "organization=1", "actions": []string{"application:read"}}),
		},
		"inheritance with both deltas": {
			grantMap(map[string]any{
				"nrn": "organization=1", "inherits": []string{"ops"},
				"add_actions": []string{"a:b"}, "remove_actions": []string{"c:d"},
			}),
		},
		// The API fills both on read; state only ever holds one, but a plan
		// against an imported resource can see both.
		"a role carrying both id and slug": {
			grantMap(map[string]any{"nrn": "organization=1", "role_slug": "ops", "role_id": 42}),
		},
	}

	for name, grants := range valid {
		t.Run(name, func(t *testing.T) {
			assert.NoError(t, validateGrantShapes(grants))
		})
	}

	invalid := map[string]struct {
		grants  []map[string]any
		message string
	}{
		"no shape at all": {
			grants:  []map[string]any{grantMap(map[string]any{"nrn": "organization=1"})},
			message: "exactly one",
		},
		"a role and actions together": {
			grants: []map[string]any{grantMap(map[string]any{
				"nrn": "organization=1", "role_slug": "ops",
				"actions": []string{"application:read"},
			})},
			message: "exactly one",
		},
		"actions and inherits together": {
			grants: []map[string]any{grantMap(map[string]any{
				"nrn": "organization=1", "actions": []string{"application:read"},
				"inherits": []string{"ops"},
			})},
			message: "exactly one",
		},
		"a delta without inherits": {
			grants: []map[string]any{grantMap(map[string]any{
				"nrn": "organization=1", "actions": []string{"application:read"},
				"add_actions": []string{"application:delete"},
			})},
			message: "add_actions",
		},
		// verifyExistanceOfRoles resolves every grant by whichever key the
		// first one used, so the mixed key fails inside the API.
		"role_id in one grant and role_slug in another": {
			grants: []map[string]any{
				grantMap(map[string]any{"nrn": "organization=1", "role_slug": "ops"}),
				grantMap(map[string]any{"nrn": "organization=2", "role_id": 42}),
			},
			message: "role_id",
		},
	}

	for name, tc := range invalid {
		t.Run(name, func(t *testing.T) {
			err := validateGrantShapes(tc.grants)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.message)
		})
	}
}

// The whole point of keeping the key-private role out of state: what the user
// wrote must survive a create-then-read cycle unchanged, or every plan that
// follows reports a change.
func TestGrantsRoundTripIsStable(t *testing.T) {
	config := []map[string]any{
		grantMap(map[string]any{"nrn": "organization=1:account=1", "role_slug": "ops"}),
		grantMap(map[string]any{
			"nrn": "organization=1:account=1:namespace=1", "actions": []string{"application:read"},
		}),
		grantMap(map[string]any{
			"nrn": "organization=1:account=1", "inherits": []string{"ops"},
			"remove_actions": []string{"deployment:create"},
		}),
	}
	require.NoError(t, validateGrantShapes(config))

	roleID := int64(9001)
	slug := "ops"
	privateSlug := "apikey:12333:0"

	// What the API answers for exactly those three grants.
	response := []ApiKeyGrantRead{
		{NRN: "organization=1:account=1", RoleID: &roleID, RoleSlug: &slug},
		{
			NRN: "organization=1:account=1:namespace=1", RoleID: &roleID, RoleSlug: &privateSlug,
			Actions: []string{"application:read"},
		},
		{
			NRN: "organization=1:account=1", RoleID: &roleID, RoleSlug: &privateSlug,
			Actions:  []string{"application:read"},
			Inherits: []ApiKeyInheritedRole{{ID: 11, Slug: "ops"}},
			Removed:  []string{"deployment:create"},
		},
	}

	for i, raw := range convertFromGrants(response, true) {
		assert.Equal(t, apiKeyGrantHash(config[i]), apiKeyGrantHash(grantMap(raw)),
			"grant %d changed identity across a round trip", i)
	}
}
