# reporter

Reporter reports drifts in local Git repositories from remote main branches, ensuring that your repositories
are current and synchronized. [How it Works](#how-it-works)

## Installation
### Prerequisites
Ensure you have Go installed on your machine. You can download it from [the official Go website](https://go.dev/dl/).

### Installing with go install
You can install the binary directly using `go install`. Follow these steps:

1. Set the environment variable for the Go path:

    ```sh
    export GOPATH=$(go env GOPATH)
    ```
2. Install the binary:

    ```sh
    go install github.com/devpies/reporter@latest
    ```

    This command will download the package, compile it, and place the binary in your `$GOPATH/bin`.


3. Ensure `$GOPATH/bin` is in your `$PATH`:

    ```sh
    export PATH=$PATH:$GOPATH/bin
    ```

    You can add this line to your shell configuration file (e.g., `~/.bashrc`, `~/.zshrc`) to make it persistent:
    
    ```sh
    echo 'export PATH=$PATH:$GOPATH/bin' >> ~/.bashrc
    echo 'alias rp=reporter' >> ~/.bashrc
    source ~/.bashrc
    ```

## Running Reporter

Run `reporter` from a parent folder containing all your git repositories:

```sh
$ rp

Checking all repositories for updates against git:(main)...

Outdated Repositories:
mvp-service is 13 commits behind (main). Last commit author: Lois Lane

Up-to-Date Repositories:
mvp-frontend is up-to-date
mvp-backend-go is up-to-date
mvp-backend-python is up-to-date
mvp-shared-library is up-to-date
mvp-tools is up-to-date

$ rp --help
Usage: rp (reporter) [OPTIONS]

A tool for reporting the status of Git repositories.
...
```

## How it works

When your project relies on many repositories with numerous developers making daily changes. It quickly
becomes difficult and frustrating to determine if your dependencies are up-to-date with the remote main branches.

`reporter` is a simple tool designed to report the status of Git repositories.
When executed within a Git repository, it checks if the local main branch is up-to-date with the remote main branch.
If the repository is behind, it fetches updates from the remote and displays detailed commit information from the
remote main branch.

This detailed information includes commit hashes, authors, dates, and commit messages, providing comprehensive
insight into what has changed upstream.

If the tool is run in a directory that is not a Git repository, it will recursively check all subdirectories to
identify and report the status of any Git repositories it finds. It categorizes these repositories as either
up-to-date or outdated based on their sync status with the remote main branch.

This functionality is particularly useful for developers working in environments with multiple repositories, as it
allows for quick verification of repository statuses and identification of necessary updates.

### Limitations

1. Reporter expects your main branch is called `main`.
2. Reporter does not run `git pull` for you.
3. Reporter is not a replacement for git.
4. Reporter only provides a short preview of the last `2` commits when used in a git repository. 
5. We expect a developer to run `git log main..origin/main` to analyze the complete list of changes before running 
`git pull`.
