package service

type Logger interface {
    Info(v ...interface{})
    Error(v ...interface{})
}

type Interface interface {
    Start(s Service) error
    Stop(s Service) error
}

type Service interface {
    Run() error
    Start() error
    Stop() error
}

type Config struct {
    Name        string
    DisplayName string
    Description string
    Option      map[string]interface{}
}

type stubService struct {
    i      Interface
    c      *Config
    stopCh chan struct{}
}

func (s *stubService) Run() error {
    if s.i != nil {
        _ = s.i.Start(s)
    }
    <-s.stopCh
    return nil
}

func (s *stubService) Start() error { return nil }
func (s *stubService) Stop() error  { close(s.stopCh); return nil }

func New(i Interface, c *Config) (Service, error) {
    return &stubService{i: i, c: c, stopCh: make(chan struct{})}, nil
}

