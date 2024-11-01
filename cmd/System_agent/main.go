package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SystemInfo holds all gathered system information
type SystemInfo struct {
	OSRelease     map[string]string
	Packages      []Package
	Configurations map[string]ConfigFile
}

type Package struct {
	Name            string
	Version         string
	ConfigFiles     []string
	RequiredPackages []string
}

type ConfigFile struct {
	Path     string
	Content  string
	Modified string
}

// OSDetector handles OS detection and package manager identification
type OSDetector struct {
	osRelease map[string]string
}

func NewOSDetector() (*OSDetector, error) {
	content, err := ioutil.ReadFile("/etc/os-release")
	if err != nil {
		return nil, fmt.Errorf("failed to read os-release: %v", err)
	}

	osRelease := make(map[string]string)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.Trim(parts[0], "\"")
			value := strings.Trim(parts[1], "\"")
			osRelease[key] = value
		}
	}

	return &OSDetector{osRelease: osRelease}, nil
}

// PackageManager handles package queries based on the detected OS
type PackageManager struct {
	pkgType string // "rpm" or "apt"
}

func NewPackageManager(osID string) *PackageManager {
	pkgType := "apt"
	if strings.Contains(strings.ToLower(osID), "rhel") || 
	   strings.Contains(strings.ToLower(osID), "centos") || 
	   strings.Contains(strings.ToLower(osID), "fedora") {
		pkgType = "rpm"
	}
	return &PackageManager{pkgType: pkgType}
}

func (pm *PackageManager) GetInstalledPackages() ([]Package, error) {
	var cmd *exec.Cmd
	if pm.pkgType == "rpm" {
		cmd = exec.Command("rpm", "-qa", "--queryformat", 
			"%{NAME}\t%{VERSION}\t%{CONFIGFILES}\n")
	} else {
		cmd = exec.Command("dpkg-query", "-W", "-f", 
			"${Package}\t${Version}\t${Conffiles}\n")
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to query packages: %v", err)
	}

	var packages []Package
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 {
			pkg := Package{
				Name:    parts[0],
				Version: parts[1],
			}
			if len(parts) > 2 {
				pkg.ConfigFiles = strings.Split(parts[2], " ")
			}
			packages = append(packages, pkg)
		}
	}

	return packages, nil
}

// ConfigurationReader reads and parses configuration files
type ConfigurationReader struct {
	rootDir string
}

func NewConfigurationReader(rootDir string) *ConfigurationReader {
	return &ConfigurationReader{rootDir: rootDir}
}

func (cr *ConfigurationReader) ReadConfigFile(path string) (*ConfigFile, error) {
	fullPath := filepath.Join(cr.rootDir, path)
	content, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %v", path, err)
	}

	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info for %s: %v", path, err)
	}

	return &ConfigFile{
		Path:     path,
		Content:  string(content),
		Modified: fileInfo.ModTime().String(),
	}, nil
}

// Agent orchestrates the system information gathering
type Agent struct {
	osDetector    *OSDetector
	pkgManager    *PackageManager
	configReader  *ConfigurationReader
	systemInfo    *SystemInfo
}

func NewAgent() (*Agent, error) {
	osDetector, err := NewOSDetector()
	if err != nil {
		return nil, err
	}

	pkgManager := NewPackageManager(osDetector.osRelease["ID"])
	configReader := NewConfigurationReader("/")

	return &Agent{
		osDetector:    osDetector,
		pkgManager:    pkgManager,
		configReader:  configReader,
		systemInfo:    &SystemInfo{},
	}, nil
}

func (a *Agent) GatherSystemInfo() error {
	// Gather OS information
	a.systemInfo.OSRelease = a.osDetector.osRelease

	// Gather package information
	packages, err := a.pkgManager.GetInstalledPackages()
	if err != nil {
		return err
	}
	a.systemInfo.Packages = packages

	// Read configurations for packages
	configs := make(map[string]ConfigFile)
	for _, pkg := range packages {
		for _, configPath := range pkg.ConfigFiles {
			config, err := a.configReader.ReadConfigFile(configPath)
			if err != nil {
				fmt.Printf("Warning: couldn't read config %s: %v\n", configPath, err)
				continue
			}
			configs[configPath] = *config
		}
	}
	a.systemInfo.Configurations = configs

	return nil
}

func main() {
	agent, err := NewAgent()
	if err != nil {
		fmt.Printf("Failed to initialize agent: %v\n", err)
		os.Exit(1)
	}

	err = agent.GatherSystemInfo()
	if err != nil {
		fmt.Printf("Failed to gather system info: %v\n", err)
		os.Exit(1)
	}

	// Output gathered information as JSON
	jsonData, err := json.MarshalIndent(agent.systemInfo, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal system info: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(jsonData))
}
