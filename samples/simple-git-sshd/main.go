package main

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/Sirupsen/logrus"
	"github.com/jessevdk/go-flags"
	"github.com/nyushi/gitrepod"
)

var opts struct {
	SSHAddr        string `short:"l" long:"ssh-addr"     description:"ssh addr"     default:"0.0.0.0"`
	SSHPort        int    `short:"p" long:"ssh-port"     description:"ssh port"     default:"22"`
	SSHHostKeyPath string `short:"i" long:"ssh-host-key" description:"ssh host key" default:"id_rsa"`
	RepoRoot       string `short:"r" long:"repo-root"    description:"repo root"    default:""`
	APIAddr        string `short:""  long:"api-addr"     description:"api addr"     default:"127.0.0.1"`
	APIPort        int    `short:""  long:"api-port"     description:"api port"     default:"3776"`
}

func main() {
	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		logrus.Fatal(err)
	}

	path, err := filepath.Abs(opts.RepoRoot)
	if err != nil {
		logrus.Fatalf("failed to get absolute path for %s: %s", opts.RepoRoot, err)
	}
	strings.TrimSuffix(path, "/")

	repod := gitrepod.GITRepoD{
		Address:    opts.SSHAddr,
		Port:       opts.SSHPort,
		APIAddress: opts.APIAddr,
		APIPort:    opts.APIPort,
		RootDir:    path,
		HostKey:    opts.SSHHostKeyPath,
		AuthHandler: func(cm ssh.ConnMetadata, pubkey ssh.PublicKey, repo string) bool {
			logrus.Infof("%s to %s", cm.User(), repo)
			return true

			/* sshkey check example
			p, _ := gitrepod.LoadSSHAuthorizedKey("/path/to/authorized_keys")
			return gitrepod.SameSSHPubkeys(pubkdy, p)
			*/
		},
		NewRevHandler: func(dir string) string {
			// this value is output of git push
			return "OK"
		},
		OverwriteGitPostReceive: true,
	}
	if err := repod.Start(); err != nil {
		logrus.Fatal(err)
	}
}
