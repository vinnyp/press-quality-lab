package record

// NextActions returns QE12’s literal table; U18 attaches actions to judge results.
func NextActions() map[string]string {
	return map[string]string{
		"network_unverifiable": "The confinement ended the subject. Possible causes: a network request, a child process, another refused operation, or an external kill. Check that the command has an offline setting that keeps it off the network, and that it starts no child process. On an unsupported system, grade on one QH1 names.",
		"could_not_start":      "Check that the binary the subject file names is an executable for this machine.",
		"input_missing":        "For the planted secret: name the file the subject reads its secret from, its token, and a command that reads it, and check that the command reads it. For another check: supply what the check needs.",
		"precondition_unmet":   "For a generated check: discovery did not find every declared command, or could not read the help; fix the declared list or the binary. Otherwise: seed the data the check needs to exist.",
		"empty_target":         "Seed data the check can evaluate, or declare the bound, the exit codes or the free-of-writes command the probe needs.",
		"placeholder_unfilled": "Fill each placeholder the scorecard names, and classify each discovered command the file does not declare.",
		"computation_failed":   "Run the computation by hand on the pristine seed and make it print exactly one JSON value within its budget, without anything its confinement refuses.",
		"earlier_step_failed":  "Fix the first step of the runbook that was not held.",
		"harness_fault":        "Rerun. If it repeats, report it with the stored record: the engine or the host failed, not the subject. When a stream was cut, the output passed the 1 MiB the engine keeps.",
		"no_evidence":          "Report it with the stored record: the engine fixed a check it did not observe.",
		"scope_not_requested":  "Request a scope that includes the dimension. For `cli-mcp-parity`, no scope includes it in this version.",
	}
}
