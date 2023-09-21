package startup

type ServiceProvider interface {
	Open() error
	Close()
}

// Service consists of a named ServiceProviders and a bunch of listeners.
type Service struct {
	name      string
	log       Logger
	service ServiceProvider
	listeners Listeners
}

func NewService(log Logger, name string, service ServiceProvider, listeners Listeners) Service {
	s := Service{
		name: name,
		service: service,
		listeners: listeners,
	}
	s.log = log.WithIndex("service", s.String())
	return s
}

func (s *Service) String() string {
	return s.name
}

func (s *Service) Open() error {
	return s.service.Open()
}

func (s *Service) Close() {
	s.service.Close()
}

func (s *Service) Listen() error {
	return s.listeners.Listen()
}
