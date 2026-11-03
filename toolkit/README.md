# Bundled Bo_cong_cu

These modules were supplied by the repository owner in `Downloads/Bo_cong_cu`.
They are embedded into the Go executable; end users do not need this directory,
Go, Node.js, or a source checkout to launch the CLI.

| Original module | Embedded file |
| --- | --- |
| Windows/2_Dong_lenh/Install_DongLenh.bat | windows/install-cli.ps1 |
| Windows/2_Dong_lenh/Uninstall_DongLenh.bat | windows/uninstall-cli.ps1 |
| Windows/1_Komorebi/Install_Komorebi.bat | windows/install-wm.ps1 |
| Windows/1_Komorebi/Uninstall_Komorebi.bat | windows/uninstall-wm.ps1 |
| macOS-Linux/2_Dong_lenh/setup.sh | unix/install-cli.sh |
| macOS-Linux/2_Dong_lenh/uninstall.sh | unix/uninstall-cli.sh |
| macOS-Linux/1_Cua_so/setup-wm.sh | unix/install-wm.sh |
| macOS-Linux/1_Cua_so/uninstall-wm.sh | unix/uninstall-wm.sh |

The Windows batch wrappers have been removed. The CLI invokes PowerShell with
`-File` and a private temporary UTF-8 BOM script, so Windows PowerShell 5.1 reads
the original Vietnamese text correctly. Unix scripts run under Bash. Temporary
scripts are removed after execution. Dry runs never extract or run scripts.

Integration changes: Windows CLI scripts accept WJ_GROUPS; package installation
failures return nonzero; replacing GlazeWM requires a separate choice; uninstalling
Komorebi no longer uninstalls the unrelated GlazeWM installation. The CLI confirms
the selected action before starting any module and strips inherited credential
and unattended WJ_* variables from the child environment.

The toolkit retains its existing configuration behavior: marked shell-profile
blocks, Git defaults/delta, terminal font/prompt, fnm Node setup, and optional
Docker. Window-manager modules change desktop/taskbar integration and shortcuts.
Linux window-manager setup configures an already installed i3 or Hyprland; it
does not install a desktop session. Uninstall can remove packages that existed
before the toolkit if the user selects those packages. Review the module prompts.

These scripts are syntax-checked in CI. Full installation/uninstallation is not
run in CI, because it changes the host desktop, packages, shell and system state.
