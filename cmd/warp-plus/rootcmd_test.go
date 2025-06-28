package main

import (
	"context"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v4"
)

// Helper function to parse args and check for a specific error message
func testParseArgsForError(t *testing.T, args []string, expectedErrorMsg string) {
	t.Helper()
	cfg := newRootCmd()
	err := cfg.command.Parse(args)

	if err == nil {
		// Try to run to catch exec validation errors
		runErr := cfg.command.Run(context.Background())
		if runErr == nil {
			t.Fatalf("expected error containing '%s' when parsing args %v, but got no error", expectedErrorMsg, args)
		}
		err = runErr // Use the error from Run for checking
	}

	if expectedErrorMsg == "" && err != nil {
		t.Fatalf("expected no error when parsing args %v, but got: %v", args, err)
	}

	if expectedErrorMsg != "" && (err == nil || !strings.Contains(err.Error(), expectedErrorMsg)) {
		actualErrStr := "nil"
		if err != nil {
			actualErrStr = err.Error()
		}
		t.Fatalf("expected error containing '%s' when parsing args %v, but got: %s", expectedErrorMsg, args, actualErrStr)
	}
}

func TestAmneziaFlagParsing(t *testing.T) {
	tests := []struct {
		name             string
		args             []string
		expectedAmnezia  bool
		expectedErrorMsg string
	}{
		{
			name:            "long amnezia flag",
			args:            []string{"--amnezia", "--endpoint", "server:1234"},
			expectedAmnezia: true,
		},
		{
			name:            "short amnezia flag",
			args:            []string{"-a", "--endpoint", "server:1234"},
			expectedAmnezia: true,
		},
		{
			name:             "amnezia without endpoint",
			args:             []string{"--amnezia"},
			expectedErrorMsg: "must provide --endpoint for AmneziaWG server",
		},
		{
			name:             "amnezia with psiphon",
			args:             []string{"--amnezia", "--cfon", "--country", "US", "--endpoint", "server:1234"},
			expectedErrorMsg: "can't use amnezia and cfon (psiphon) at the same time",
		},
		{
			name:             "amnezia with gool",
			args:             []string{"--amnezia", "--gool", "--endpoint", "server:1234"},
			expectedErrorMsg: "can't use amnezia and gool (warp-in-warp) at the same time",
		},
		{
			name:             "amnezia with wgconf",
			args:             []string{"--amnezia", "--wgconf", "config.conf", "--endpoint", "server:1234"},
			expectedErrorMsg: "can't use amnezia and wgconf (direct wireguard config) at the same time",
		},
		{
			name:             "amnezia with scan",
			args:             []string{"--amnezia", "--scan", "--endpoint", "server:1234"},
			expectedErrorMsg: "can't use amnezia and scan mode at the same time",
		},
		{
			name:            "no amnezia flag",
			args:            []string{"--endpoint", "server:1234"},
			expectedAmnezia: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedErrorMsg != "" {
				// For tests expecting an error during Run/validation phase
				// We check the error message directly from the validation logic in exec
				// The Parse itself might not error for some of these.

				// Create a dummy exec for testing validation, as the actual exec starts goroutines.
				// The validation happens before goroutines in the original exec.
				cfg := newRootCmd()
				originalExec := cfg.command.Exec
				cfg.command.Exec = func(ctx context.Context, args []string) error {
					// This is where the validation logic from the original exec would run.
					// We are testing if cfg.amnezia and other flags are set correctly by Parse
					// and then if the validation logic (which we replicated parts of for testing) catches issues.
					// Here we directly call the original exec to test its internal validation.
					return originalExec(ctx, args)
				}

				err := cfg.command.Parse(tt.args)
				if errors.Is(err, ff.ErrHelp) { // Allow help errors to pass if no specific message
					if tt.expectedErrorMsg != "" {
						t.Fatalf("expected error containing '%s', but got help error", tt.expectedErrorMsg)
					}
					return
				}
				if err != nil && tt.expectedErrorMsg == "" {
					t.Fatalf("unexpected parsing error: %v", err)
					return
				}
				if err == nil && tt.expectedErrorMsg != "" {
					// If Parse was successful, call Run to trigger validation in Exec
					runErr := cfg.command.Run(context.Background())
					if runErr == nil {
						t.Fatalf("expected error containing '%s' from exec, but got no error", tt.expectedErrorMsg)
					}
					if !strings.Contains(runErr.Error(), tt.expectedErrorMsg) {
						t.Fatalf("expected error from exec containing '%s', but got: %v", tt.expectedErrorMsg, runErr)
					}
					return
				}
				// If Parse itself errored, and we expected an error
				if err != nil && tt.expectedErrorMsg != "" {
					if !strings.Contains(err.Error(), tt.expectedErrorMsg) {
						// This case might be tricky if the error comes from ff parser vs. our validation
						// For now, let's assume our validation errors are the primary target.
						// If ff parser catches it first (e.g. bad flag value type), that's also a valid error.
						// The helper testParseArgsForError is better for Run-time validation errors.
						// Let's refine this if it becomes problematic.
						// For now, if any error occurs and we expect one, we check containment.
						t.Logf("Parse error: %v. Checking if it contains expected message.", err)
						if !strings.Contains(err.Error(), tt.expectedErrorMsg) && !errors.Is(err, ff.ErrHelp) {
                             // If Parse error is not help and not what we expected, call Run to get validation error
                            runErr := cfg.command.Run(context.Background())
                            if runErr == nil || !strings.Contains(runErr.Error(), tt.expectedErrorMsg) {
                                t.Fatalf("expected error containing '%s', but Parse gave '%v' and Run gave '%v'", tt.expectedErrorMsg, err, runErr)
                            }
                        }
					}
				}

			} else {
				// For tests expecting successful parsing and correct flag values
				cfg := newRootCmd()
				err := cfg.command.Parse(tt.args)
				if err != nil && !errors.Is(err, ff.ErrHelp) {
					t.Fatalf("unexpected error when parsing args %v: %v", tt.args, err)
				}
				if errors.Is(err, ff.ErrHelp) && tt.expectedErrorMsg == "" && len(tt.args) > 0 && tt.args[0] != "-h" && tt.args[0] != "--help" {
					// ff can return ErrHelp if unknown flags are present, even if not explicitly asking for help.
					// This might happen if a flag used in one test case (e.g. --cfon) is not reset properly for the next.
					// This indicates a potential issue with test isolation or ff's global state if not careful.
					// However, for simple cases, if we expect success and get help, it's a failure.
					t.Fatalf("expected successful parse for %v, but got help error", tt.args)
				}


				if cfg.amnezia != tt.expectedAmnezia {
					t.Errorf("for args %v, expected amnezia %v, but got %v", tt.args, tt.expectedAmnezia, cfg.amnezia)
				}
			}
		})
	}
}

[end of cmd/warp-plus/rootcmd_test.go]
