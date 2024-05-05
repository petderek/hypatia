package hypatia

import (
	"errors"
	"os"
)

// Example

type HealthCheck interface {
	GetHealth() error
	SetHealth(bool) error
}

// FileHealthcheck is a healthcheck that fails if the filepath requested doesn't exist, and succeeds if a file does
// exist at that location. Default location is 'health.status'
type FileHealthcheck struct {
	Filepath string
}

func (hc *FileHealthcheck) GetHealth() error {
	if _, err := os.Stat(hc.filename()); err != nil {
		return errors.New("file [" + hc.filename() + "] not found")
	}
	return nil
}

func (hc *FileHealthcheck) SetHealth(status bool) error {
	if status {
		return hc.enable()
	}
	return hc.disable()
}

func (hc *FileHealthcheck) enable() error {
	f, err := os.Stat(hc.filename())
	if err == nil && f.Name() != "" {
		// file already exists
		return nil
	}
	c, err := os.Create(hc.filename())
	if err != nil {
		return err
	}
	return c.Close()
}

func (hc *FileHealthcheck) disable() error {
	_, err := os.Stat(hc.filename())
	if err != nil {
		// file does not exist
		return nil
	}
	return os.Remove(hc.filename())
}

const defaultHealthFile = "health.status"

func (hc *FileHealthcheck) filename() string {
	if hc.Filepath == "" {
		return defaultHealthFile
	}
	return hc.Filepath
}
