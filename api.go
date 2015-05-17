package gitrepod

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
)

func (g *GITRepoD) apiHook(w http.ResponseWriter, r *http.Request) {
	rev, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("failed to read body: %s", err)
		w.WriteHeader(500)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/hook")
	path, err = filepath.Abs(path)
	if err != nil {
		logrus.Errorf("failed to get absolute path for %s: %s", path, err)
		w.WriteHeader(400)
	}
	repo := gitRepo{g.RootDir, path}
	dir, err := repo.Checkout(string(rev))
	if err != nil {
		logrus.Errorf("failed to checkout %s in %s:%s", rev, path, err)
		w.WriteHeader(400)
		return
	}
	msg := g.NewRevHandler(dir)
	fmt.Fprintf(w, "%s", msg)
}

func (g *GITRepoD) apiCreate(w http.ResponseWriter, r *http.Request) {
	var err error

	path := strings.TrimPrefix(r.URL.Path, "/create")
	path, err = filepath.Abs(path)
	if err != nil {
		logrus.Errorf("failed to get absolute path for %s: %s", path, err)
		w.WriteHeader(400)
	}
	repo := gitRepo{g.RootDir, path}
	if repo.Ready() {
		logrus.Infof("repo %s is already exists", path)
		w.WriteHeader(409)
		return
	}
	if err := repo.Init(g.APIAddress, g.APIPort); err != nil {
		logrus.Errorf("failed to create repo: %s", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(200)
	return
}

func (g *GITRepoD) startAPIServer() {
	m := http.NewServeMux()
	m.HandleFunc("/hook/", g.apiHook)
	m.HandleFunc("/create/", g.apiCreate)
	s := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", g.APIAddress, g.APIPort),
		Handler:        m,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	logrus.Error(s.ListenAndServe())
}
