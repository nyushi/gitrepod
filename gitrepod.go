package gitrepod

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/flynn/go-shlex"

	"golang.org/x/crypto/ssh"
)

type session struct {
	Pubkey ssh.PublicKey
}

// GITRepoD is git repository server
type GITRepoD struct {
	Address                 string
	Port                    int
	APIAddress              string
	APIPort                 int
	HostKey                 string
	RootDir                 string
	AuthHandler             func(conn ssh.ConnMetadata, key ssh.PublicKey, repo string) bool
	NewRevHandler           func(string) string
	OverwriteGitPostReceive bool

	pubkeys map[string]ssh.PublicKey
}

// Start starts daemon
func (g *GITRepoD) Start() error {
	logrus.Infof("repository root: %s", g.RootDir)
	if err := g.prepareRepos(); err != nil {
		return err
	}
	go g.startAPIServer()
	return g.startSSHD()
}

func (g *GITRepoD) prepareRepos() error {
	err := filepath.Walk(g.RootDir, func(path string, info os.FileInfo, err error) error {
		p := strings.TrimPrefix(path, g.RootDir)
		repo := gitRepo{g.RootDir, p}
		if repo.Ready() {
			current := repo.GetCurrentPostReceiveScript()
			should := repo.GetPostReceiveScript(g.APIAddress, g.APIPort)
			if bytes.Compare(current, should) != 0 {
				if !g.OverwriteGitPostReceive {
					logrus.Fatalf("git post receive script mismatch in %s\n%s\n\nAND\n\n%s", path, current, should)
				}
			}
			return filepath.SkipDir
		}
		return nil
	})
	return err
}
func (g *GITRepoD) startSSHD() error {
	g.pubkeys = map[string]ssh.PublicKey{}
	conf := g.sshConfig()
	l := fmt.Sprintf("%s:%d", g.Address, g.Port)
	listener, err := net.Listen("tcp", l)
	logrus.Infof("Listen %s", l)
	if err != nil {
		return fmt.Errorf("failed to listen for connection: %s", err)
	}

	for {
		nConn, err := listener.Accept()
		if err != nil {
			logrus.Info("failed to accept incoming connection")
			continue
		}
		raddr := nConn.RemoteAddr().String()

		conn, chans, reqs, err := ssh.NewServerConn(nConn, conf)
		if err != nil {
			logrus.Infof("failed to handshake from %s", raddr)
			continue
		}
		logrus.Infof("accepted ssh connection from %s", raddr)

		go ssh.DiscardRequests(reqs)

		go func() {
			for ch := range chans {
				if ch.ChannelType() != "session" {
					ch.Reject(ssh.UnknownChannelType, "unknown channel type")
					logrus.Infof("invalid channel type request `%s` from %s", ch.ChannelType(), raddr)
					continue
				}
				go g.handleSession(conn, ch)
			}
		}()
	}
}

func (g *GITRepoD) sshConfig() *ssh.ServerConfig {
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(cm ssh.ConnMetadata, pubkey ssh.PublicKey) (*ssh.Permissions, error) {
			g.pubkeys[cm.RemoteAddr().String()] = pubkey
			return nil, nil
		},
	}
	s, err := loadSSHPrivateKey(g.HostKey)
	if err != nil {
		logrus.Warningf("failed to load ssh host key: %s", err)
		s, err = genSSHPrivateKey(g.HostKey)
	}
	if err != nil {
		logrus.Fatalf("failed to generate ssh host key: %s", err)
	}
	config.AddHostKey(s)
	return config
}

func (g *GITRepoD) handleSession(conn *ssh.ServerConn, newChan ssh.NewChannel) {
	ch, reqs, err := newChan.Accept()
	if err != nil {
		logrus.Infof("newChan.Accept failed: %s", err)
		return
	}
	defer ch.Close()
	for req := range reqs {
		switch req.Type {
		case "exec":
			b := true
			if err := g.handlePush(conn, req, ch); err != nil {
				logrus.Infof("handlePush failed: %s", err)
				b = false
			}
			req.Reply(b, nil)
			return
		case "env":
			if req.WantReply {
				req.Reply(true, nil)
			}
		default:
			req.Reply(false, nil)
		}
	}
}

func (g *GITRepoD) handlePush(conn *ssh.ServerConn, req *ssh.Request, ch ssh.Channel) error {
	if req.WantReply {
		req.Reply(true, nil)
	}
	reqArgs, _ := shlex.Split(string(req.Payload[4:]))
	if (len(reqArgs) != 2) ||
		(reqArgs[0] != "git-receive-pack") {
		return errors.New("invalid exec argments")
	}

	reqPath := reqArgs[1]
	reqPath, err := filepath.Abs(reqPath)
	if err != nil {
		return fmt.Errorf("invalid repository %s: %s", reqPath, err)
	}
	if !strings.HasPrefix(reqPath, "/") {
		reqPath = "/" + reqPath
	}

	addr := conn.RemoteAddr().String()
	pubkey := g.pubkeys[addr]
	delete(g.pubkeys, addr)

	if !g.AuthHandler(conn, pubkey, reqPath) {
		return errors.New("unauthorized")
	}

	repo := gitRepo{g.RootDir, reqPath}
	if !repo.Ready() {
		return fmt.Errorf("%s is not initialized", reqPath)
	}
	cmd := exec.Command("git-shell", "-c", fmt.Sprintf(`git-receive-pack '%s'`, repo.FullPath()))
	cmd.Dir = repo.FullPath()
	cmd.Env = append(os.Environ(),
		"RECEIVE_USER="+conn.User(),
		"RECEIVE_REPO="+repo.FullPath(),
	)

	logrus.Info("run git-shell")
	if err := doCommand(cmd, ch, ch, ch.Stderr()); err != nil {
		return err
	}
	var status struct {
		Status uint32
	}
	if _, err := ch.SendRequest("exit-status", false, ssh.Marshal(&status)); err != nil {
		return fmt.Errorf("failed to ch.SendRequest: %s", err)
	}
	return nil
}
