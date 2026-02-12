package scripting

import (
	"sigs.k8s.io/yaml"
)

type CloudConfigUser struct {
	Name              string   `json:"name,omitempty"`
	Gecos             string   `json:"gecos,omitempty"`
	Groups            string   `json:"groups,omitempty"`
	Shell             string   `json:"shell,omitempty"`
	Sudo              string   `json:"sudo,omitempty"`
	LockPasswd        *bool    `json:"lock_passwd,omitempty"`
	SshAuthorizedKeys []string `json:"ssh_authorized_keys,omitempty"`
	Passwd            string   `json:"passwd,omitempty"`
}

type CloudConfigFile struct {
	Path        string `json:"path"`
	Content     string `json:"content"`
	Owner       string `json:"owner,omitempty"`
	Permissions string `json:"permissions,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	Append      bool   `json:"append,omitempty"`
	Defer       bool   `json:"defer,omitempty"`
}

type CloudConfig struct {
	Users             []CloudConfigUser `json:"users,omitempty"`
	Groups            []string          `json:"groups,omitempty"`
	WriteFiles        []CloudConfigFile `json:"write_files,omitempty"`
	RunCmd            [][]string        `json:"runcmd,omitempty"`
	SshPwauth         *bool             `json:"ssh_pwauth,omitempty"`
	DisableRoot       *bool             `json:"disable_root,omitempty"`
	SshAuthorizedKeys []string          `json:"ssh_authorized_keys,omitempty"`
}

func (c *CloudConfig) Serialize() ([]byte, error) {
	// Use sigs.k8s.io/yaml to marshal the struct.
	// This library is a wrapper around go-yaml/yaml and handles JSON tags correctly,
	// but it produces YAML.
	// By default, it might not use block scalars (|) for multiline strings,
	// but standard YAML parsers (like cloud-init) handle quoted multiline strings correctly too.
	// If you specifically need the '|' style for readability, you'd need a custom marshaler or a different library,
	// but functionally, the output from sigs.k8s.io/yaml is valid YAML and cloud-init will accept it.

	data, err := yaml.Marshal(c)
	if err != nil {
		return nil, err
	}
	return append([]byte("#cloud-config\n"), data...), nil
}

//
//func GenerateCloudInit(config *crossplane.CloudInit) ([]byte, error) {
//	if config == nil {
//		return nil, fmt.Errorf("cloud-init config is nil")
//	}
//
//	cc := &CloudConfig{
//		Groups: config.GetGroups(),
//	}
//	for _, cmd := range config.GetRuncmd() {
//		cc.RunCmd = append(cc.RunCmd, cmd.GetValues())
//	}
//
//	for _, u := range config.GetUsers() {
//		cu := CloudConfigUser{
//			Name:              u.GetName(),
//			Gecos:             u.GetGecos(),
//			Shell:             u.GetShell(),
//			Sudo:              u.GetSudoRules(),
//			SshAuthorizedKeys: u.GetSshAuthorizedKeys(),
//			Passwd:            u.GetPasswd(),
//		}
//		if len(u.GetGroups()) > 0 {
//			cu.Groups = strings.Join(u.GetGroups(), ",")
//		}
//		if u.LockPasswd {
//			t := true
//			cu.LockPasswd = &t
//		}
//		cc.Users = append(cc.Users, cu)
//	}
//
//	for _, f := range config.GetWriteFiles() {
//		cc.WriteFiles = append(cc.WriteFiles, CloudConfigFile{
//			Path:        f.GetPath(),
//			Content:     f.GetContent(),
//			Owner:       f.GetOwner(),
//			Permissions: f.GetPermissions(),
//			Encoding:    f.GetEncoding(),
//			Append:      f.GetAppend(),
//			Defer:       f.GetDefer(),
//		})
//	}
//
//	if ssh := config.GetSsh(); ssh != nil {
//		if len(ssh.GetSshAuthorizedKeys()) > 0 {
//			cc.SshAuthorizedKeys = ssh.GetSshAuthorizedKeys()
//		}
//		if ssh.DisableRoot {
//			t := true
//			cc.DisableRoot = &t
//		}
//		if ssh.EmitKeysToConsole {
//			// Not supported in this struct yet, but could be added if needed
//		}
//	}
//
//	return cc.Serialize()
//}
