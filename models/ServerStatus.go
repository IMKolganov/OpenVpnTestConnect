package models

type ServerStatus struct {
	Config  VPNConfig
	Success bool
	Output  string
	Error   string
}
