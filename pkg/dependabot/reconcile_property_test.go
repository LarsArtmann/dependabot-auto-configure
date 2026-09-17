package dependabot_test

import (
	"math/rand"
	"reflect"
	"slices"
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

// deepCopyConfig round-trips a config through its wire format, producing a
// copy that shares no pointers with the original.
func deepCopyConfig(t *testing.T, cfg dependabot.Config) dependabot.Config {
	t.Helper()

	raw, err := cfg.Encode()
	if err != nil {
		t.Fatalf("deep-copy source does not encode: %v", err)
	}

	res, err := dependabot.Decode(raw)
	if err != nil {
		t.Fatalf("deep-copy redecode failed: %v", err)
	}

	return res.Config
}

// TestReconcilePreservesEveryModeledField is the adversarial counterpart to
// the idempotence properties: idempotence constrains only the second call,
// so a lossy first write (the v0.2.0 schedule-fill bug) passed both. Each
// iteration reconciles a randomized user config against a pristine deep
// copy and demands, entry by entry, that no modeled field was dropped —
// fields may only be filled where they were empty.
func TestReconcilePreservesEveryModeledField(t *testing.T) {
	t.Parallel()

	const iterations = 500

	rng := rand.New(rand.NewSource(20260918))

	for i := range iterations {
		existing := randomExistingConfig(rng)
		desired := randomDesiredConfig(rng)

		pristine := deepCopyConfig(t, existing)

		out := dependabot.Reconcile(existing, desired)

		if len(out.Updates) < len(pristine.Updates) {
			t.Fatalf("iteration %d: reconcile dropped entries: %d -> %d", i, len(pristine.Updates), len(out.Updates))
		}

		for j, want := range pristine.Updates {
			got := out.Updates[j]

			if got.PackageEcosystem != want.PackageEcosystem || got.Directory != want.Directory {
				t.Fatalf(
					"iteration %d: entry %d identity changed: %s/%s -> %s/%s",
					i,
					j,
					want.PackageEcosystem,
					want.Directory,
					got.PackageEcosystem,
					got.Directory,
				)
			}

			if !slices.Equal(got.Labels, want.Labels) {
				t.Fatalf(
					"iteration %d: entry %s/%s labels changed: %v -> %v",
					i,
					want.PackageEcosystem,
					want.Directory,
					want.Labels,
					got.Labels,
				)
			}

			idx := desired.Find(want.PackageEcosystem, want.Directory)

			switch {
			case want.Schedule == nil && idx < 0 && got.Schedule != nil:
				t.Fatalf("iteration %d: orphan entry %s/%s grew a schedule", i, want.PackageEcosystem, want.Directory)

			case want.Schedule != nil:
				if got.Schedule == nil {
					t.Fatalf("iteration %d: entry %s/%s lost its schedule", i, want.PackageEcosystem, want.Directory)
				}

				if got.Schedule.Day != want.Schedule.Day ||
					got.Schedule.Time != want.Schedule.Time ||
					got.Schedule.Timezone != want.Schedule.Timezone {
					t.Fatalf(
						"iteration %d: entry %s/%s schedule customizations changed: %+v -> %+v",
						i,
						want.PackageEcosystem,
						want.Directory,
						want.Schedule,
						got.Schedule,
					)
				}

				if want.Schedule.Interval != "" && got.Schedule.Interval != want.Schedule.Interval {
					t.Fatalf(
						"iteration %d: entry %s/%s interval replaced: %q -> %q",
						i,
						want.PackageEcosystem,
						want.Directory,
						want.Schedule.Interval,
						got.Schedule.Interval,
					)
				}

				if want.Schedule.Interval == "" && got.Schedule.Interval != dependabot.IntervalWeekly {
					t.Fatalf(
						"iteration %d: entry %s/%s empty interval filled to %q, want weekly",
						i,
						want.PackageEcosystem,
						want.Directory,
						got.Schedule.Interval,
					)
				}
			}

			switch {
			case want.OpenPullRequestsLimit != 0 && got.OpenPullRequestsLimit != want.OpenPullRequestsLimit:
				t.Fatalf(
					"iteration %d: entry %s/%s limit replaced: %d -> %d",
					i,
					want.PackageEcosystem,
					want.Directory,
					want.OpenPullRequestsLimit,
					got.OpenPullRequestsLimit,
				)

			case want.OpenPullRequestsLimit == 0 && idx >= 0 && got.OpenPullRequestsLimit != desired.Updates[idx].OpenPullRequestsLimit:
				t.Fatalf(
					"iteration %d: entry %s/%s zero limit not filled from desired: got %d, want %d",
					i,
					want.PackageEcosystem,
					want.Directory,
					got.OpenPullRequestsLimit,
					desired.Updates[idx].OpenPullRequestsLimit,
				)

			case want.OpenPullRequestsLimit == 0 && idx < 0 && got.OpenPullRequestsLimit != 0:
				t.Fatalf(
					"iteration %d: orphan entry %s/%s grew limit %d",
					i,
					want.PackageEcosystem,
					want.Directory,
					got.OpenPullRequestsLimit,
				)
			}

			switch {
			case want.Groups == nil && idx < 0 && got.Groups != nil:
				t.Fatalf("iteration %d: orphan entry %s/%s grew groups", i, want.PackageEcosystem, want.Directory)

			case want.Groups != nil && !reflect.DeepEqual(got.Groups, want.Groups):
				t.Fatalf(
					"iteration %d: entry %s/%s groups replaced: %+v -> %+v",
					i,
					want.PackageEcosystem,
					want.Directory,
					want.Groups,
					got.Groups,
				)
			}
		}
	}
}
