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
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = pkgs.stdenv.isLinux;
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
        ExecStartPre = "${pkgs.coreutils}/bin/mkdir -p ${mountPath}";
        ExecStart = "${cfg.package}/bin/task-fs mount ${mountPath}";
        ExecStopPost = "-${pkgs.fuse3}/bin/fusermount3 -u ${mountPath}";
        Restart = "on-failure";
        Environment = [
          "PATH=${
            lib.makeBinPath [
              pkgs.taskwarrior3
              pkgs.fuse3
            ]
          }"
        ];
      };
    };
  };
}
