package config_test

import (
	"testing"

	"github.com/kcaldas/genie/pkg/config"
	geniectx "github.com/kcaldas/genie/pkg/ctx"
	"github.com/stretchr/testify/assert"
)

func TestDefaultConfiguredModelExistsInRegistry(t *testing.T) {
	t.Setenv("GENIE_MODEL_NAME", "")
	model := config.NewConfigManager().GetModelConfig().ModelName
	_, ok := geniectx.LookupModelInfo(model)
	assert.Truef(t, ok, "default model %q has no registry entry", model)
}
