{
  config,
  lib,
  pkgs,
  taskFsFlake,
  ...
}:

let
  cfg = config.services.task-fs;

  mountPath =
    let
      path = cfg.automount.path;
    in
    if lib.hasPrefix "~" path then config.home.homeDirectory + lib.removePrefix "~" path else path;
in
{
  options.services.task-fs = {
    enable = lib.mkEnableOption "TaskFS, the Taskwarrior filesystem";

    package = lib.mkOption {
      type = lib.types.package;
      default = taskFsFlake.packages.${pkgs.system}.default;
      defaultText = lib.literalExpression ''
        inputs.task-fs.packages.''${pkgs.system}.default
      '';
      description = "The task-fs package to install and to mount with.";
    };

    automount = {
      enable = lib.mkEnableOption "automounting the Taskwarrior filesystem at {option}`services.task-fs.automount.path` on session start";

      path = lib.mkOption {
        type = lib.types.str;
        default = "";
        example = "~/some/path";
        description = ''
          Mountpoint for the Taskwarrior filesystem. A leading `~` expands to
          the user's home directory.
        '';
      };

      mountFlags = lib.mkOption {
        type = lib.types.listOf lib.types.str;
        default = [ ];
        example = [
          "--rm"
          "delete"
        ];
        description = ''
          Flags passed to `task-fs mount` by the automount service. Each
          item is one command-line token. For example, `--rm delete` makes
          removal gestures destructive. Every mount already exposes the
          `+COMPLETED` and `+DELETED` status views.

          Custom views are created through the mounted tree with
          `mkdir @NAME` and a filter written to `@NAME/_filter`; they are not
          mount flags. Their definitions persist under the service user's
          `$XDG_STATE_HOME/task-fs/views`, falling back to
          `$HOME/.local/state/task-fs/views`, so they survive service
          restarts. `@NAME` directories already present under the automount
          path are imported when the service starts.

          `{{ ... }}` blocks in a `_filter` run in the automount service
          environment, which provides a POSIX shell and coreutils. Commands
          outside that baseline must be referenced by absolute path or
          through a wrapper script.
        '';
      };

      extraPackages = lib.mkOption {
        type = lib.types.listOf lib.types.package;
        default = [ ];
        example = lib.literalExpression "[ pkgs.delta.cycle ]";
        description = ''
          Packages to add to the automount service `PATH`, so commands in a
          `{{ ... }}` block can be invoked by name. For example, adding
          `pkgs.delta.cycle` lets a `_filter` use `{{cycle last week --end}}`.
          The default `PATH` still provides taskwarrior3, fuse3, and coreutils.
        '';
      };
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = pkgs.stdenv.hostPlatform.isLinux;
        message = "services.task-fs: TaskFS supports Linux only (it requires Linux FUSE).";
      }
      {
        assertion = !cfg.automount.enable || cfg.automount.path != "";
        message = "services.task-fs.automount.path must be set when services.task-fs.automount.enable is true.";
      }
    ];

    home.packages = [ cfg.package ];

    systemd.user.services.task-fs = lib.mkIf cfg.automount.enable {
      Unit = {
        Description = "TaskFS - the Taskwarrior filesystem";
        After = [ "default.target" ];
      };

      Install = {
        WantedBy = [ "default.target" ];
      };

      Service = {
        # systemd resolves bare command names with the manager's PATH,
        # not the unit's Environment, so fusermount3 must be absolute;
        # /run/wrappers/bin carries NixOS's setuid wrapper. The guard
        # skips the unmount on a clean start, when nothing is mounted.
        ExecStartPre = [
          "-/bin/sh -c '${pkgs.util-linux}/bin/mountpoint -q ${mountPath} && /run/wrappers/bin/fusermount3 -u ${mountPath}'"
          "${pkgs.coreutils}/bin/mkdir -p ${mountPath}"
        ];
        ExecStart = "${cfg.package}/bin/task-fs mount ${
          lib.escapeShellArgs (cfg.automount.mountFlags ++ [ mountPath ])
        }";
        ExecStopPost = "-/run/wrappers/bin/fusermount3 -u ${mountPath}";
        Restart = "on-failure";
        Environment = [
          "PATH=/run/wrappers/bin:${
            lib.makeBinPath (
              [
                pkgs.taskwarrior3
                pkgs.fuse3
                pkgs.coreutils
              ]
              ++ cfg.automount.extraPackages
            )
          }"
        ];
      };
    };
  };
}
