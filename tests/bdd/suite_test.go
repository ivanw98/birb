//go:build bdd

package bdd

import (
	"context"
	"os"
	"testing"

	"github.com/cucumber/godog"
)

// TestFeatures is the suite's entry point (`go test -tags bdd ./tests/bdd/...`); it skips rather than fails when no test database is configured.
func TestFeatures(t *testing.T) {
	dsn := os.Getenv("BIRB_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("BIRB_TEST_DATABASE_URL not set; see tests/bdd/README.md to start a disposable Postgres and run this suite")
	}

	h, err := newHarness(context.Background(), dsn)
	if err != nil {
		t.Fatalf("bdd harness setup: %v", err)
	}
	t.Cleanup(h.close)

	// Scenarios tagged @WIP are skipped by default; set BIRB_BDD_TAGS (e.g. "@WIP") to override the filter.
	tags := "~@WIP"
	if v := os.Getenv("BIRB_BDD_TAGS"); v != "" {
		tags = v
	}

	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario(h),
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
			Strict:   true,
			Tags:     tags,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("one or more BDD scenarios failed")
	}
}
