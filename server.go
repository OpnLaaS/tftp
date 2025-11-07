package tftp

func NewServer(options Options) (server *Server, err error) {
	server = &Server{
		Options:     options,
		QuitChannel: make(chan struct{}),
	}

	return
}

func (s *Server) Stop() {
	close(s.QuitChannel)
}

func (s *Server) Get(getType GetType, ) {}