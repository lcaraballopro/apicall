package sysadmin

import (
	"bufio"
	"os"
	"strings"
)

type OSType int

const (
	Unknown OSType = iota
	Debian
	RHEL
	Suse
)

// DetectOS attempts to determine the OS family
func DetectOS() OSType {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return Unknown
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID_LIKE=") || strings.HasPrefix(line, "ID=") {
			line = strings.ToLower(line)
			if strings.Contains(line, "debian") || strings.Contains(line, "ubuntu") {
				return Debian
			}
			if strings.Contains(line, "rhel") || strings.Contains(line, "centos") || strings.Contains(line, "fedora") {
				return RHEL
			}
			if strings.Contains(line, "suse") {
				return Suse
			}
		}
	}
	return Unknown
}
