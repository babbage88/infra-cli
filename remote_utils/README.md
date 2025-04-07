# Remote Utilities

This director is used to hold the internal compiled binaries that are copied to and executed on remote hosts via sftp/ssh.

If an internal package is named main and has a main function, it will be compiles seperate from the primay module (infractl) binary.

Will probaby embed them via the embed package to the main infractl binary.

## Usage

These utils can be called manually from bash with their own command flags. They are meant to be self contained and seperate from the primary Viper config.

The intended usecase is for the primary application to copy them over sftp to a remote host and then execute via ssh, using the goph package.

If there is an established way to perform the action remotely without ssh, I will default to that. But, to avoid the main infractl application calling long chains of bash commands and to try to standardize exit codes.

As an example, go has the os/user package for managing users, but no method in the package to perform on remote hosts. So to keep the benefits of using os/user, like error handling, we can create a remote_util implementatio of the local validate username and uid functions used in systemd based app deployment.
