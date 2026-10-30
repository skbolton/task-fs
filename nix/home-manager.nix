{
  config,
  lib,
  pkgs,
  taskFsFlake,
  ...
}:

let
  cfg = config.services.task-fs;

  expandHome =
    path:
    if lib.hasPrefix "~" path then config.home.homeDirectory + lib.removePrefix "~" path else path;

  mountUnit =
    name: mount:
    let
      mountPath = expandHome mount.path;
      taskData = if mount.taskData == null then null else expandHome mount.taskData;
    in
    lib.nameValuePair "task-fs-${name}" {
      Unit = {
        Description = "TaskFS - the Taskwarrior filesystem (${name})";
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
          lib.escapeShellArgs (mount.mountFlags ++ [ mountPath ])
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
              ++ mount.extraPackages
            )
          }"
        ]
        ++ lib.optional (taskData != null) "TASKDATA=${taskData}";
      };
    };
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

    mounts = lib.mkOption {
      type = lib.types.attrsOf (
        lib.types.submodule {
          options = {
            path = lib.mkOption {
              type = lib.types.str;
              example = "~/some/path";
              description = ''
                Mountpoint for this Taskwarrior filesystem. A leading `~`
                expands to the user's home directory.
              '';
            };

            taskData = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              example = "~/task-data/work";
              description = ''
                Taskwarrior data directory for this mount. When set, the
                service runs with `TASKDATA` pointing here; when null, it
                inherits the systemd user manager environment. A leading `~`
                expands to the user's home directory.
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
                Flags passed to `task-fs mount` by this mount's service. Each
                item is one command-line token. For example, `--rm delete`
                makes removal gestures destructive. Every mount already
                exposes the `+COMPLETED` and `+DELETED` status views.

                Custom views are created through the mounted tree with
                `mkdir @NAME` and a definition written to `@NAME/_view`; they
                are not mount flags. The first line is the filter. An optional
                second line holds the taskwarrior modify arguments applied
                when a task is moved into the view; without it the view is
                read-only. Definitions persist under the service user's
                `$XDG_STATE_HOME/task-fs/views`, falling back to
                `$HOME/.local/state/task-fs/views`, so they survive service
                restarts. `@NAME` directories already present under the mount
                path are imported when the service starts.

                `{{ ... }}` blocks in either line of `_view` run in this
                mount's service environment, which provides a POSIX shell and
                coreutils. Commands outside that baseline must be referenced by
                absolute path or through a wrapper script.
              '';
            };

            extraPackages = lib.mkOption {
              type = lib.types.listOf lib.types.package;
              default = [ ];
              example = lib.literalExpression "[ pkgs.delta.cycle ]";
              description = ''
                Packages to add to this mount service's `PATH`, so commands in
                a `{{ ... }}` block can be invoked by name. For example, adding
                `pkgs.delta.cycle` lets a `_view` use
                `{{cycle last week --end}}`. The default `PATH` still provides
                taskwarrior3, fuse3, and coreutils.
              '';
            };
          };
        }
      );
      default = { };
      description = ''
        Named Taskwarrior filesystem mounts. Each attribute key names one
        mount and its systemd user service, `task-fs-<name>.service`.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = pkgs.stdenv.hostPlatform.isLinux;
        message = "services.task-fs: TaskFS supports Linux only (it requires Linux FUSE).";
      }
    ]
    ++ lib.concatLists (
      lib.mapAttrsToList (name: mount: [
        {
          assertion = mount.path != "";
          message = "services.task-fs.mounts.${name}.path must not be empty.";
        }
        {
          assertion = builtins.match "[A-Za-z0-9_-]+" name != null;
          message = "services.task-fs.mounts.${name}: mount names may contain only letters, digits, hyphens, and underscores.";
        }
        {
          assertion = mount.taskData == null || mount.taskData != "";
          message = "services.task-fs.mounts.${name}.taskData must not be empty when set.";
        }
      ]) cfg.mounts
    );

    home.packages = [ cfg.package ];

    systemd.user.services = lib.mapAttrs' mountUnit cfg.mounts;
  };
}
