package diag

// TODO v1.0: Stub diagnostics; v1.1: MTU test; v1.2: STUN test.

type Result struct {
    STUNOK bool
    MTUOK  bool
    Notes  string
}

func Run() (*Result, error) {
    return &Result{STUNOK: true, MTUOK: true, Notes: "stub"}, nil
}

