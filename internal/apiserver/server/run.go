package server

import "context"

// Run 启动 ingate-apiserver
func (s *Server) Run(ctx context.Context) error {
	return s.GenericAPIServer.PrepareRun().RunWithContext(ctx)
}
