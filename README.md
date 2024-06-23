# reporter

Reporter recursively reports and resolves drifts across multiple git repositories. [How it Works](#how-it-works)

```
$ rp
Checking all repositories for updates against git: main

Outdated Repositories:
mvp-service is 13 commits behind (main). Last commit: Lois Lane
(hash: abc123, date: Fri Nov 24 10:56:42 2023 +0100) - fix: provide db transaction context

Up-to-Date Repositories:
mvp-frontend is up-to-date
mvp-backend-go is up-to-date
mvp-backend-python is up-to-date
mvp-shared-library is up-to-date
mvp-tools is up-to-date
````

## Installation
### Prerequisites
Ensure you have [Git](https://git-scm.com/downloads) and [Go](https://go.dev/dl/) installed on your machine.

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

### Installing from Source

You can also install reporter by cloning the repository and building it from source. Follow these steps:

```sh
git clone https://github.com/devpies/reporter
cd reporter
go build -o rp reporter.go
sudo mv rp /usr/local/bin/rp
```

## Getting Started

### Reporting Multiple Git Repositories

To maximize the tool's effectiveness, run reporter in a parent directory that contains multiple Git repositories. 
This approach allows for efficient management and synchronization of all repositories simultaneously.

```
$ rp
Checking all repositories for updates against git: main

Outdated Repositories:
mvp-service is 13 commits behind (main). Last commit: Lois Lane
(hash: abc123, date: Fri Nov 24 10:56:42 2023 +0100) - fix: provide db transaction context

Up-to-Date Repositories:
mvp-frontend is up-to-date
mvp-backend-go is up-to-date
mvp-backend-python is up-to-date
mvp-shared-library is up-to-date
mvp-tools is up-to-date
````

### Updating Multiple Git Repositories

To automatically update repositories that are behind:

```
$ rp --update

Checking all repositories for updates against git: main

Outdated Repositories:
mvp-service is 13 commits behind (main). Last commit: Lois Lane
(hash: abc123, date: Fri Nov 24 10:56:42 2023 +0100) - fix: provide db transaction context
Stashing local changes...
Pulling latest changes...
Applying stashed changes...
Repository updated successfully!

Up-to-Date Repositories:
mvp-frontend is up-to-date
mvp-backend-go is up-to-date
mvp-backend-python is up-to-date
mvp-shared-library is up-to-date
mvp-tools is up-to-date
```

### Using the --log Flag

In a Git repository, you can show the complete list of changes in the default remote branch.

```
$ rp --log

commit abc123
Author: Lois Lane
Date:   Fri Nov 24 10:56:42 2023 +0100

    fix: provide db transaction context

commit def456
Author: Clark Kent
Date:   Mon Dec 01 10:56:42 2023 +0100

    feat: add new authentication module
```

### Using the --branch Flag
To check for updates against a different branch:

```
$ rp --branch develop

Checking all repositories for updates against git: develop

Outdated Repositories:
mvp-service is 7 commits behind (develop). Last commit: Clark Kent
(hash: def456, date: Mon Dec 01 10:56:42 2023 +0100) - feat: add new authentication module

Up-to-Date Repositories:
mvp-frontend is up-to-date
mvp-backend-go is up-to-date
mvp-backend-python is up-to-date
mvp-shared-library is up-to-date
mvp-tools is up-to-date
```

### Using the --branch and --update Flags Together

To check for updates against a different branch and automatically update repositories that are behind:

```
$ rp --branch develop --update

Checking all repositories for updates against git: develop

Outdated Repositories:
mvp-service is 7 commits behind (develop). Last commit: Clark Kent
(hash: def456, date: Mon Dec 01 10:56:42 2023 +0100) - feat: add new authentication module
Stashing local changes...
Pulling latest changes...
Applying stashed changes...
Repository updated successfully!

Up-to-Date Repositories:
mvp-frontend is up-to-date
mvp-backend-go is up-to-date
mvp-backend-python is up-to-date
mvp-shared-library is up-to-date
mvp-tools is up-to-date
```

## Configuration File (.rprc)

The `.rprc` file is an optional YAML configuration file that allows you to customize the behavior of the reporter tool.
It can specify the branch to check for updates, whether to automatically update repositories that are behind, and which
repositories to include or exclude from the check.

For optimal use, place the `.rprc` file in the parent directory containing all your Git repositories. This allows the
configuration to be applied to multiple repositories managed by reporter.

### Include/Exclude Repositories

You can specify which repositories to include or exclude in the `.rprc` file.

- **Include List:** If you specify an include list, only the repositories listed will be checked.
- **Exclude List:** If you specify an exclude list, the repositories listed will be ignored.
- **Combination:** If both lists are specified, the tool will check only the repositories listed in include and will
- exclude any repositories that are also listed in exclude. The exclude list refines the include list by removing
- repositories that should not be checked.

Example `.rprc` File

```yaml
branch: main
update: true
include:
   - repo1
   - repo2
   - repo3
exclude:
   - repo3
```

## Help

To display usage information:

```
$ rp --help

Usage: rp (reporter) [OPTIONS]

Reporter recursively reports and resolves drifts across multiple git repositories.

Options:
--help, -h        Show this help message
--update, -u      Automatically update repositories that are behind
--branch, -b      Specify the branch to check (default: main)
--log, -l         Show the complete list of changes using git log

Examples:

[Truncated Output For Brevity]
...
```

## How it works

When executed in a directory that is not a Git repository, it will recursively check all subdirectories to identify
and report the status of any Git repositories it finds. It categorizes these repositories as either up-to-date or
outdated based on their sync status with the remote main branch. Optionally, it can also automatically update 
repositories that are behind by stashing local changes, pulling the latest updates, and reapplying the stashed changes.

When executed within a Git repository, it checks if the local main branch is up-to-date with the remote main branch.
If the repository is behind, it fetches updates from the remote and displays detailed commit information from the remote
main branch. This detailed information includes commit hashes, authors, dates, and commit messages, providing
comprehensive insight into what has changed upstream.

This functionality is particularly useful for developers working in environments with multiple repositories, as it
allows for quick verification of repository statuses and identification of necessary updates. 

### Unique Benefits:
1. **Automation and Convenience**: Automates the process of checking and updating multiple repositories, saving time and
reducing manual errors.
2. **Batch Processing**: Recursively checks and updates multiple repositories in a single command, unlike using Git 
commands individually for each repository.
3. **Centralized Configuration**: The `.rprc` configuration file allows users to specify settings like branches to 
check, whether to auto-update, and which repositories to include or exclude.
4. **Detailed Reporting**: Provides comprehensive commit information, including commit hashes, authors, dates, and 
messages.
5. **Selective Updates**: Allows selective checking and updating of specific repositories via include/exclude lists.
6. **Stashing and Applying Changes**: Automatically stashes local changes, pulls the latest updates, and reapplies the 
stashed changes.
