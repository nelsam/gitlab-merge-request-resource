package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	resource "github.com/samcontesse/gitlab-merge-request-resource"
	"github.com/samcontesse/gitlab-merge-request-resource/common"
	"github.com/samcontesse/gitlab-merge-request-resource/in"
	"github.com/xanzy/go-gitlab"
)

func setupSSHKey(src resource.Source) error {
	if src.PrivateKey == "" {
		return nil
	}
	sshDir := filepath.Join(os.Getenv("HOME"), ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("failed to create ssh directory: %w", err)
	}
	privKeyPath := filepath.Join(sshDir, "privkey")
	f, err := os.Create(privKeyPath)
	if err != nil {
		return fmt.Errorf("failed to create ssh key file: %w", err)
	}
	defer f.Close()

	if err := f.Chmod(0600); err != nil {
		return fmt.Errorf("failed to change permissions of private key file: %w", err)
	}
	key := []byte(src.PrivateKey)
	for len(key) > 0 {
		n, err := f.Write(key)
		if err != nil {
			return fmt.Errorf("failed to write private key to file: %w", err)
		}
		key = key[n:]
	}

	cfgPath := filepath.Join(sshDir, "config")
	cfg, err := os.Create(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to create ssh config file: %w", err)
	}
	defer cfg.Close()
	b := []byte(fmt.Sprintf(`
StrictHostKeyChecking no
LogLevel quiet

Host gitlab.com
    IdentityFile %s
`, privKeyPath))
	for len(b) > 0 {
		n, err := cfg.Write(b)
		if err != nil {
			return fmt.Errorf("failed to write ssh config: %w", err)
		}
		b = b[n:]
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		println("usage: " + os.Args[0] + " <destination>")
		os.Exit(1)
	}

	destination := os.Args[1]

	var request in.Request

	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		common.Fatal("reading request from stdin", err)
	}

	api := gitlab.NewClient(common.GetDefaultClient(request.Source.Insecure), request.Source.PrivateToken)
	api.SetBaseURL(request.Source.GetBaseURL())

	mr, _, err := api.MergeRequests.GetMergeRequest(request.Source.GetProjectPath(), request.Version.ID, &gitlab.GetMergeRequestsOptions{})

	if err != nil {
		common.Fatal("getting merge request", err)
	}

	mr.UpdatedAt = request.Version.UpdatedAt

	if err := setupSSHKey(request.Source); err != nil {
		common.Fatal("setting up ssh private key", err)
	}

	target := createRepositoryUrl(api, mr.TargetProjectID, request.Source.PrivateToken, request.Source.PrivateKey != "")
	source := createRepositoryUrl(api, mr.SourceProjectID, request.Source.PrivateToken, request.Source.PrivateKey != "")

	commit, _, err := api.Commits.GetCommit(mr.SourceProjectID, mr.SHA)
	if err != nil {
		common.Fatal("listing merge request commits", err)
	}

	execGitCommand([]string{"clone", "-c", "http.sslVerify=" + strconv.FormatBool(!request.Source.Insecure), "-o", "target", "-b", mr.TargetBranch, target, destination})
	os.Chdir(destination)
	execGitCommand([]string{"remote", "add", "source", source})
	execGitCommand([]string{"remote", "update"})
	execGitCommand([]string{"merge", "--no-ff", "--no-commit", mr.SHA})

	if request.Source.Submodules != "none" && fileExists(".gitmodules") {
		f, err := os.Create(filepath.Join(os.Getenv("HOME"), ".netrc"))
		if err != nil {
			common.Fatal("Creating .netrc", err)
		}
		for _, cred := range request.Source.SubmoduleCreds {
			f.Write([]byte(fmt.Sprintf("machine %s login %s password %s\n", cred.Host, cred.Username, cred.Password)))
		}
		execGitCommand([]string{"submodule", "update", "--quiet", "--init", "--recursive"})
	}

	notes, _ := json.Marshal(mr)
	err = ioutil.WriteFile(".git/merge-request.json", notes, 0644)

	response := in.Response{Version: request.Version, Metadata: buildMetadata(mr, commit)}

	json.NewEncoder(os.Stdout).Encode(response)
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func execGitCommand(args []string) {
	execCommand("git", args)
}

func execCommand(cmd string, args []string) {
	command := exec.Command(cmd, args...)
	command.Stdin = os.Stdin
	command.Stderr = os.Stderr
	err := command.Run()
	if err != nil {
		common.Fatal("executing "+cmd+" "+strings.Join(args, " "), err)
	}
}

func createRepositoryUrl(api *gitlab.Client, pid int, token string, useSSH bool) string {
	project, _, err := api.Projects.GetProject(pid, &gitlab.GetProjectOptions{})
	if err != nil {
		common.Fatal("reading project from api", err)
	}
	if useSSH {
		return project.SSHURLToRepo
	}

	u, err := url.Parse(project.HTTPURLToRepo)
	if err != nil {
		common.Fatal("parsing repository http url", err)
	}
	u.User = url.UserPassword("gitlab-ci-token", token)
	return u.String()
}

func buildMetadata(mr *gitlab.MergeRequest, commit *gitlab.Commit) resource.Metadata {
	return []resource.MetadataField{
		{
			Name:  "id",
			Value: strconv.Itoa(mr.ID),
		},
		{
			Name:  "iid",
			Value: strconv.Itoa(mr.IID),
		},
		{
			Name:  "sha",
			Value: mr.SHA,
		},
		{
			Name:  "message",
			Value: commit.Title,
		},
		{
			Name:  "title",
			Value: mr.Title,
		},
		{
			Name:  "author",
			Value: mr.Author.Name,
		},
		{
			Name:  "source",
			Value: mr.SourceBranch,
		},
		{
			Name:  "target",
			Value: mr.TargetBranch,
		},
		{
			Name:  "url",
			Value: mr.WebURL,
		},
	}
}
