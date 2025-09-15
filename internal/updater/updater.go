package updater

// TODO v1.0: Stub self-update; v1.1+: implement real update flow.

type Result struct {
    Available bool
    Version   string
    Notes     string
}

func Check() (*Result, error) {
    return &Result{Available: false, Version: "v1.0.0", Notes: "stub"}, nil
}

func Apply() error { return nil }

