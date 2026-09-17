package dependabot_test

import (
	"math/rand"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/dependabot"
)

// randomExistingConfig builds a plausible user config: canonical entries,
// entries missing canonical fields, entries with custom schedules or
// labels, and orphan entries for undetected directories.
func randomExistingConfig(rng *rand.Rand) dependabot.Config {
	ecosystems := []dependabot.Ecosystem{
		dependabot.EcosystemGoModules,
		dependabot.EcosystemGitHubActions,
		dependabot.EcosystemNPM,
	}
	directories := []string{"/", "/modules/types", "/packages/app", "/tools/legacy"}

	cfg := dependabot.Config{Version: dependabot.CurrentVersion}

	for range rng.Intn(4) {
		update := dependabot.Update{
			PackageEcosystem: ecosystems[rng.Intn(len(ecosystems))],
			Directory:        directories[rng.Intn(len(directories))],
		}

		if rng.Intn(2) == 0 {
			sched := &dependabot.Schedule{Interval: dependabot.IntervalWeekly}
			if rng.Intn(3) == 0 {
				sched.Day = "monday"
			}

			update.Schedule = sched
		}

		if rng.Intn(2) == 0 {
			update.OpenPullRequestsLimit = rng.Intn(10) + 1
		}

		if rng.Intn(2) == 0 {
			update.Groups = dependabot.MinorAndPatchGroups()
		}

		if rng.Intn(4) == 0 {
			update.Labels = []string{"dependencies"}
		}

		cfg.Updates = append(cfg.Updates, update)
	}

	return cfg
}

func randomDesiredConfig(rng *rand.Rand) dependabot.Config {
	shape := dependabot.RepoShape{
		GoModuleDirs:     []string{"", "/modules/types"},
		HasGitHubActions: rng.Intn(2) == 0,
		HasNPM:           rng.Intn(2) == 0,
	}

	desired, _ := dependabot.Generate(shape)

	return desired
}

// TestReconcileIsIdempotent is a seeded randomized property: reconciling an
// already-reconciled config again must not change it. Repair converges, or
// it is not repair.
func TestReconcileIsIdempotent(t *testing.T) {
	t.Parallel()

	const iterations = 500

	rng := rand.New(rand.NewSource(20260917))

	for i := range iterations {
		existing := randomExistingConfig(rng)
		desired := randomDesiredConfig(rng)

		once := dependabot.Reconcile(existing, desired)
		twice := dependabot.Reconcile(once, desired)

		if !dependabot.Equal(once, twice) {
			t.Fatalf(
				"iteration %d: Reconcile did not converge\nexisting: %+v\ndesired: %+v\nonce:     %+v\ntwice:    %+v",
				i,
				existing,
				desired,
				once,
				twice,
			)
		}

		if err := once.Validate(); err != nil {
			t.Fatalf("iteration %d: reconciled config has invalid entries: %v\nonce: %+v", i, err, once)
		}
	}
}

// TestReconcileIsIdempotentOnGenerated pins the special case the account
// relies on: the canonical desired config is a fixed point of Reconcile.
func TestReconcileIsIdempotentOnGenerated(t *testing.T) {
	t.Parallel()

	shape := dependabot.RepoShape{GoModuleDirs: []string{"", "/modules/types"}, HasGitHubActions: true, HasNPM: true}
	desired, _ := dependabot.Generate(shape)

	if got := dependabot.Reconcile(desired, desired); !dependabot.Equal(got, desired) {
		t.Fatalf("generated config is not a fixed point:\ndesired: %+v\ngot:     %+v", desired, got)
	}

	if _, err := desired.Encode(); err != nil {
		t.Fatalf("generated config does not encode: %v", err)
	}
}
