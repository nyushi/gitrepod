package gitrepod

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
)

type gitRepo struct {
	RootDir string
	Path    string
}

func (r *gitRepo) FullPath() string {
	return r.RootDir + r.Path
}

func (r *gitRepo) Ready() bool {
	s, err := os.Stat(r.FullPath())
	if err != nil {
		logrus.Debugf("stat error for %s: %s", r.FullPath(), err)
		return false
	}
	if !s.IsDir() {
		logrus.Debugf("%s is not dir", r.FullPath())
		return false
	}
	cmd := exec.Command("git", "rev-parse", "--is-bare-repository")
	cmd.Dir = r.FullPath()
	b, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Debugf("git rev-parse --is-bare-repository for %s: %s", r.FullPath(), err)
		return false
	}
	return bytes.HasPrefix(b, []byte("true"))
}

func (r *gitRepo) GetPostReceiveScript(addr string, port int) []byte {
	if addr == "0.0.0.0" {
		addr = "127.0.0.1"
	}

	var script = `
while read oldrev newrev refname; do
  curl -sS -d "${newrev}" "http://%s:%d/hook%s"
  echo
done
`
	return []byte(fmt.Sprintf(script, addr, port, r.Path))

}

func (r *gitRepo) GetCurrentPostReceiveScript() []byte {
	scriptPath := filepath.Join(r.FullPath(), "hooks", "post-receive")
	b, err := ioutil.ReadFile(scriptPath)
	if err != nil {
		return nil
	}
	return b
}

func (r *gitRepo) Init(addr string, port int) error {
	if err := os.MkdirAll(r.FullPath(), 0755); err != nil {
		return err
	}
	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = r.FullPath()
	_, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	logrus.Infof("create repository %s(%s)", r.Path, r.FullPath())
	scriptPath := filepath.Join(r.FullPath(), "hooks", "post-receive")
	return ioutil.WriteFile(scriptPath, r.GetPostReceiveScript(addr, port), 0755)
}

func (r *gitRepo) Checkout(rev string) (string, error) {
	logrus.Info(rev)
	gitCmd := exec.Command("git", "archive", rev)
	gitCmd.Dir = r.FullPath()
	gitCmd.Stderr = &bytes.Buffer{}

	tarCmd := exec.Command("tar", "-xf", "-")
	tarCmd.Dir = fmt.Sprintf("%s%s/%s", strings.TrimSuffix(os.TempDir(), "/"), r.Path, rev)
	tarCmd.Stderr = &bytes.Buffer{}

	if err := os.MkdirAll(tarCmd.Dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create tar dir: %s", err)
	}
	logrus.Info(tarCmd.Dir)

	tarData, err := gitCmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get tar stdout: %s", err)
	}
	tarCmd.Stdin = tarData

	if err := gitCmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start git archive: %s", err)
	}
	if err := tarCmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start tar: %s", err)
	}

	if err := gitCmd.Wait(); err != nil {
		return "", fmt.Errorf("failed to finish git archive: %s, %s", err, gitCmd.Stderr.(*bytes.Buffer).String())
	}
	if err := tarCmd.Wait(); err != nil {
		return "", fmt.Errorf("failed to finish tar: %s, %s", err, tarCmd.Stderr.(*bytes.Buffer).String())
	}
	return tarCmd.Dir, nil
}
