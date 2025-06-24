package cli

import (
	"testing"
	
	"github.com/kcaldas/genie/pkg/genie"
)

func TestAskCommand(t *testing.T) {
	t.Run("should work with mocked fixture", func(t *testing.T) {
		// Create fixture with mocked LLM - should be fast
		fixture := genie.NewTestFixture(t)
		fixture.ExpectSimpleMessage("test prompt", "mocked response")
		
		// Use the fixture's mocked Genie
		cmd := NewAskCommandWithGenie(func() (genie.Genie, *genie.Session) {
			session := fixture.StartAndGetSession()
			return fixture.Genie, session
		})

		cmd.SetArgs([]string{"test prompt"})

		err := cmd.Execute()
		if err != nil {
			t.Errorf("Expected mocked command to work, got error: %v", err)
		}
	})
}
