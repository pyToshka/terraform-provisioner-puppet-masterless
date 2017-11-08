package main

import (
	"fmt"
	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/communicator/remote"
	"github.com/hashicorp/terraform/terraform"
	"github.com/mitchellh/go-linereader"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Provisioner struct {
	Facter          map[string]string
	HieraConfigPath string   `mapstructure:"hiera_config_path"`
	ModulePaths     []string `mapstructure:"module_paths"`
	ManifestFile    string   `mapstructure:"manifest_file"`
	ManifestDir     string   `mapstructure:"manifest_dir"`
	PreventSudo     bool     `mapstructure:"prevent_sudo"`
	StagingDir      string   `mapstructure:"staging_directory"`
	CleanStagingDir bool     `mapstructure:"clean_staging_directory"`
	WorkingDir      string   `mapstructure:"working_directory"`
	PuppetBinDir    string   `mapstructure:"puppet_bin_dir"`
	IgnoreExitCodes bool     `mapstructure:"ignore_exit_codes"`
}

type ExecuteTemplate struct {
	WorkingDir      string
	FacterVars      string
	HieraConfigPath string
	ModulePath      string
	ManifestFile    string
	ManifestDir     string
	PuppetBinDir    string
	Sudo            bool
	ExtraArguments  string
	path            string
}

type guestOSTypeConfig struct {
	stagingDir       string
	facterVarsFmt    string
	facterVarsJoiner string
	modulePathJoiner string
}

const (
	agent_url             = "https://raw.githubusercontent.com/pyToshka/puppet-install-shell/master/install_puppet_agent.sh"
	script                = "/tmp/install.sh"
	remoteHieraConfigPath = ""
	stagingDir            = "/tmp/terraform-puppet-masterless"
	facterVarsFmt         = "FACTER_%s='%s'"
	facterVarsJoiner      = " "
	modulePathJoiner      = ":"
)

func (p *Provisioner) Run(o terraform.UIOutput, comm communicator.Communicator) error {

	if p.StagingDir == "" {
		p.StagingDir = stagingDir
	}

	if p.WorkingDir == "" {
		p.WorkingDir = p.StagingDir
	}

	if p.Facter == nil {
		p.Facter = make(map[string]string)
	}
	if p.HieraConfigPath != "" {
		info, err := os.Stat(p.HieraConfigPath)
		if err != nil {
			err = fmt.Errorf("hiera_config_path is invalid: %s", err)
		} else if info.IsDir() {
			err = fmt.Errorf("hiera_config_path must point to a file")
		}
	}

	if p.ManifestDir != "" {
		info, err := os.Stat(p.ManifestDir)
		if err != nil {
			err = fmt.Errorf("manifest_dir is invalid: %s", err)
		} else if !info.IsDir() {
			err = fmt.Errorf("manifest_dir must point to a directory")
		}
	}

	for i, path := range p.ModulePaths {
		info, err := os.Stat(path)
		if err != nil {
			err = fmt.Errorf("module_path[%d] is invalid: %s", i, err)
		} else if !info.IsDir() {
			err = fmt.Errorf("module_path[%d] must point to a directory", i)
		}
	}

	command := fmt.Sprintf("'curl %s -o %s -s'", agent_url, script)
	if err := p.runCommand(o, comm, command); err != nil {
		return err
	}

	return nil
}

func (p *Provisioner) Validate() error {

	return nil
}
func (p *Provisioner) fixPerm(o terraform.UIOutput, comm communicator.Communicator) error {
	err := p.runCommand(o, comm, fmt.Sprintf("'chmod +x %s'", script))
	if err != nil {
		return err
	}
	return nil
}
func (p *Provisioner) installPuppetAgent(o terraform.UIOutput, comm communicator.Communicator) error {
	err := p.runCommand(o, comm, fmt.Sprintf("'%s'", script))
	if err != nil {
		return err
	}
	return nil
}
func (p *Provisioner) AddPuppetAgentPath(o terraform.UIOutput, comm communicator.Communicator) error {
	err := p.runCommand(o, comm, fmt.Sprintf("'echo \"export PATH=/opt/puppetlabs/bin:$PATH\" >> /root/.bashrc && source /root/.bashrc'"))
	if err != nil {
		return err
	}
	return nil
}

func (p *Provisioner) runCommand(
	o terraform.UIOutput,
	comm communicator.Communicator,
	command string) error {
	var err error
	if p.PreventSudo {
		command = "sudo -i bash -c " + command
	}
	o.Output(command)
	outR, outW := io.Pipe()
	errR, errW := io.Pipe()
	outDoneCh := make(chan struct{})
	errDoneCh := make(chan struct{})

	go p.copyOutput(o, outR, outDoneCh)
	go p.copyOutput(o, errR, errDoneCh)

	cmd := &remote.Cmd{
		Command: command,
		Stdout:  outW,
		Stderr:  errW,
	}

	if err := comm.Start(cmd); err != nil {
		return fmt.Errorf("Error executing command %q: %v", cmd.Command, err)
	}
	cmd.Wait()
	if cmd.ExitStatus != 0 {
		err = fmt.Errorf(
			"Command %q exited with non-zero exit status: %d", cmd.Command, cmd.ExitStatus)
	}

	outW.Close()
	errW.Close()
	<-outDoneCh
	<-errDoneCh

	return err
}

func (p *Provisioner) copyOutput(o terraform.UIOutput, r io.Reader, doneCh chan<- struct{}) {
	defer close(doneCh)
	lr := linereader.New(r)
	for line := range lr.Ch {
		o.Output(line)
	}
}
func (p *Provisioner) uploadHieraConfig(o terraform.UIOutput, comm communicator.Communicator) (string, error) {
	o.Output("Uploading hiera configuration...")
	f, err := os.Open(p.HieraConfigPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	path := fmt.Sprintf("%s/hiera.yaml", p.StagingDir)
	if err := comm.Upload(path, f); err != nil {
		return "", err
	}

	return path, nil
}
func (p *Provisioner) createDir(o terraform.UIOutput, comm communicator.Communicator, dir string) error {
	o.Output(fmt.Sprintf("Creating directory: %s", dir))

	cmd := &remote.Cmd{
		Command: fmt.Sprintf("mkdir -p '%s'", dir),
	}

	if err := comm.Start(cmd); err != nil {
		return err
	}

	if cmd.ExitStatus != 0 {
		return fmt.Errorf("Non-zero exit status.")
	}
	// Chmod the directory to 0777 just so that we can access it as our user
	cmd = &remote.Cmd{
		Command: fmt.Sprintf("chmod 777 '%s'", dir),
	}
	if cmd.ExitStatus != 0 {
		return fmt.Errorf("Non-zero exit status. See output above for more info.")
	}

	return nil
}

func (p *Provisioner) Provision(o terraform.UIOutput, comm communicator.Communicator) error {
	o.Output("Provisioning with Puppet...")
	o.Output("Creating Puppet staging directory...")
	if err := p.createDir(o, comm, p.StagingDir); err != nil {
		return fmt.Errorf("Error creating staging directory: %s", err)
	}

	// Upload hiera config if set
	remoteHieraConfigPath := ""
	if p.HieraConfigPath != "" {
		var err error
		remoteHieraConfigPath, err = p.uploadHieraConfig(o, comm)
		if err != nil {
			return fmt.Errorf("Error uploading hiera config: %s", err)
		}
	}

	// Upload manifest dir if set
	remoteManifestDir := "/tmp"
	if p.ManifestDir != "" {
		o.Output(fmt.Sprintf(
			"Uploading manifest directory from: %s", p.ManifestDir))
		remoteManifestDir = fmt.Sprintf("%s/manifests", p.StagingDir)
		err := p.uploadDirectory(o, comm, remoteManifestDir, p.ManifestDir)
		if err != nil {
			return fmt.Errorf("Error uploading manifest dir: %s", err)
		}
	}

	// Upload all modules
	modulePaths := make([]string, 0, len(p.ModulePaths))
	for i, path := range p.ModulePaths {
		o.Output(fmt.Sprintf("Uploading local modules from: %s", path))
		targetPath := fmt.Sprintf("%s/module-%d", p.StagingDir, i)
		if err := p.uploadDirectory(o, comm, targetPath, path); err != nil {
			return fmt.Errorf("Error uploading modules: %s", err)
		}

		modulePaths = append(modulePaths, targetPath)
	}
	// Upload manifests
	remoteManifestFile, err := p.uploadManifests(o, comm)
	if err != nil {
		return fmt.Errorf("Error uploading manifests: %s", err)
	}

	// Compile the facter variables
	facterVars := make([]string, 0, len(p.Facter))
	for k, v := range p.Facter {
		facterVars = append(facterVars, fmt.Sprintf(facterVarsFmt, k, v))
	}

	// Execute Puppet
	ModulePath := strings.Join(modulePaths, modulePathJoiner)
	HieraConfigPath := remoteHieraConfigPath
	if HieraConfigPath != "" {
		return err
	}

	//FacterVars:=strings.Join(facterVars, p.guestOSTypeConfig.facterVarsJoiner)
	WorkingDir := p.WorkingDir
	command := fmt.Sprintf("'cd %s ; /opt/puppetlabs/bin/puppet apply --modulepath %s %s --verbose'", WorkingDir, ModulePath, remoteManifestFile)
	err = p.runCommand(o, comm, command)
	if err != nil {
		return err
	}
	o.Output(fmt.Sprintf("Running Puppet: %s", command))
	if err != nil {
		return err
	}
	if p.CleanStagingDir {
		if err := p.removeDir(o, comm, p.StagingDir); err != nil {
			return fmt.Errorf("Error removing staging directory: %s", err)
		}
	}

	return nil
}
func (p *Provisioner) uploadManifests(o terraform.UIOutput, comm communicator.Communicator) (string, error) {
	// Create the remote manifests directory...
	o.Output("Uploading manifests...")
	remoteManifestsPath := fmt.Sprintf("%s/manifests", p.StagingDir)
	if err := p.createDir(o, comm, remoteManifestsPath); err != nil {
		return "", fmt.Errorf("Error creating manifests directory: %s", err)
	}
	// NOTE! manifest_file may either be a directory or a file, as puppet apply
	// now accepts either one.

	fi, err := os.Stat(p.ManifestFile)
	if err != nil {
		return "", fmt.Errorf("Error inspecting manifest file: %s", err)
	}

	if fi.IsDir() {
		// If manifest_file is a directory we'll upload the whole thing
		o.Output(fmt.Sprintf(
			"Uploading manifest directory from: %s", p.ManifestFile))

		remoteManifestDir := fmt.Sprintf("%s/manifests", p.StagingDir)
		err := p.uploadDirectory(o, comm, remoteManifestDir, p.ManifestFile)
		if err != nil {
			return "", fmt.Errorf("Error uploading manifest dir: %s", err)
		}
		return remoteManifestDir, nil
	}
	// Otherwise manifest_file is a file and we'll upload it
	o.Output(fmt.Sprintf(
		"Uploading manifest file from: %s", p.ManifestFile))

	f, err := os.Open(p.ManifestFile)
	if err != nil {
		return "", err
	}
	defer f.Close()

	manifestFilename := filepath.Base(p.ManifestFile)
	remoteManifestFile := fmt.Sprintf("%s/%s", remoteManifestsPath, manifestFilename)
	if err := comm.Upload(remoteManifestFile, f); err != nil {
		return "", err
	}
	return remoteManifestFile, nil
}
func (p *Provisioner) uploadDirectory(
	o terraform.UIOutput, comm communicator.Communicator, dst string, src string) error {
	if err := p.createDir(o, comm, dst); err != nil {
		return err
	}
	if src[len(src)-1] != '/' {
		src = src + "/"
	}

	return comm.UploadDir(dst, src)
}
func (p *Provisioner) removeDir(o terraform.UIOutput, comm communicator.Communicator, dir string) error {
	cmd := &remote.Cmd{
		Command: fmt.Sprintf("rm -fr '%s'", dir),
	}
	if err := comm.Start(cmd); err != nil {
		return err
	}

	if cmd.ExitStatus != 0 {
		return fmt.Errorf("Non-zero exit status.")
	}

	return nil
}
