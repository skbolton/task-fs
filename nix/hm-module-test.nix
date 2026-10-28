{
  lib,
  pkgs,
  module,
  wrappedModule,
  flakePackage,
  taskFsFlake,
}:

let
  linuxPkgs = pkgs // {
    stdenv = pkgs.stdenv // {
      hostPlatform = pkgs.stdenv.hostPlatform // {
        isLinux = true;
      };
    };
  };

  stubs = {
    options = {
      home.homeDirectory = lib.mkOption {
        type = lib.types.str;
        default = "/home/testuser";
      };
      home.packages = lib.mkOption {
        type = lib.types.listOf lib.types.package;
        default = [ ];
      };
      systemd.user.services = lib.mkOption {
        type = lib.types.attrsOf lib.types.anything;
        default = { };
      };
      assertions = lib.mkOption {
        type = lib.types.listOf lib.types.anything;
        default = [ ];
      };
    };
  };

  eval =
    {
      config ? { },
      pkgsOverride ? linuxPkgs,
    }:
    lib.evalModules {
      modules = [
        module
        stubs
        {
          inherit config;
        }
      ];
      specialArgs = {
        pkgs = pkgsOverride;
        inherit taskFsFlake;
      };
    };

  flakePackage = taskFsFlake.packages.${pkgs.system}.default;

  default = eval {
    config.services.task-fs = {
      enable = true;
      automount = {
        enable = true;
        path = "~/some/path";
      };
    };
  };
  unit = default.config.systemd.user.services.task-fs;

  customPkg = pkgs.hello;
  custom = eval {
    config.services.task-fs = {
      enable = true;
      package = customPkg;
      automount = {
        enable = true;
        path = "~/some/path";
      };
    };
  };

  absolute = eval {
    config.services.task-fs = {
      enable = true;
      automount = {
        enable = true;
        path = "/mnt/tasks";
      };
    };
  };

  flagged = eval {
    config.services.task-fs = {
      enable = true;
      automount = {
        enable = true;
        path = "~/some/path";
        mountFlags = [
          "--rm"
          "delete"
        ];
      };
    };
  };

  disabled = eval { };

  wrapped = lib.evalModules {
    modules = [
      wrappedModule
      stubs
      {
        config.services.task-fs = {
          enable = true;
          automount = {
            enable = true;
            path = "~/some/path";
          };
        };
      }
    ];
    specialArgs = { inherit pkgs; };
  };
  wrappedUnit = wrapped.config.systemd.user.services.task-fs;

  nonLinux = eval {
    pkgsOverride = pkgs // {
      stdenv = pkgs.stdenv // {
        hostPlatform = pkgs.stdenv.hostPlatform // {
          isLinux = false;
        };
      };
    };
    config.services.task-fs = {
      enable = true;
      automount = {
        enable = true;
        path = "~/some/path";
      };
    };
  };

  checks = [
    {
      cond = lib.any (
        p:
        lib.hasPrefix "-" p
        && lib.hasInfix "mountpoint -q /home/testuser/some/path" p
        && lib.hasInfix "/run/wrappers/bin/fusermount3 -u /home/testuser/some/path" p
      ) unit.Service.ExecStartPre;
      msg = "ExecStartPre does not clear a stale mount best-effort: ${builtins.toJSON unit.Service.ExecStartPre}";
    }
    {
      cond = lib.any (p: lib.hasInfix "mkdir -p /home/testuser/some/path" p) unit.Service.ExecStartPre;
      msg = "ExecStartPre does not mkdir the ~-expanded mount path: ${builtins.toJSON unit.Service.ExecStartPre}";
    }
    {
      cond = lib.hasInfix "/bin/task-fs mount /home/testuser/some/path" unit.Service.ExecStart;
      msg = "ExecStart does not mount the ~-expanded mount path with the flake package: ${unit.Service.ExecStart}";
    }
    {
      cond = lib.hasPrefix (toString flakePackage) unit.Service.ExecStart;
      msg = "ExecStart does not use the flake package by default: ${unit.Service.ExecStart}";
    }
    {
      cond = lib.hasInfix "/run/wrappers/bin/fusermount3 -u /home/testuser/some/path" unit.Service.ExecStopPost;
      msg = "ExecStopPost does not unmount the mount path via the absolute wrapper path: ${unit.Service.ExecStopPost}";
    }
    {
      cond = lib.hasPrefix "-" unit.Service.ExecStopPost;
      msg = "ExecStopPost does not tolerate an already-unmounted mountpoint: ${unit.Service.ExecStopPost}";
    }
    {
      cond = unit.Service.Restart == "on-failure";
      msg = "Service does not restart on failure";
    }
    {
      cond = unit.Install.WantedBy == [ "default.target" ];
      msg = "Unit is not wanted by default.target";
    }
    {
      cond =
        unit.Service.Environment == [
          "PATH=/run/wrappers/bin:${
            lib.makeBinPath [
              pkgs.taskwarrior3
              pkgs.fuse3
              pkgs.coreutils
            ]
          }"
        ];
      msg = "Unit PATH does not prefer the setuid wrapper and provide taskwarrior3, fuse3, and coreutils";
    }
    {
      cond = lib.elem flakePackage default.config.home.packages;
      msg = "Enabling does not install the flake package";
    }
    {
      cond = lib.all (a: a.assertion) default.config.assertions;
      msg = "Default Linux evaluation has failing assertions";
    }
    {
      cond = lib.hasPrefix (toString customPkg) custom.config.systemd.user.services.task-fs.Service.ExecStart;
      msg = "A custom services.task-fs.package does not propagate into the unit: ${custom.config.systemd.user.services.task-fs.Service.ExecStart}";
    }
    {
      cond = lib.elem customPkg custom.config.home.packages;
      msg = "A custom services.task-fs.package is not installed";
    }
    {
      cond = lib.hasInfix "mount /mnt/tasks" absolute.config.systemd.user.services.task-fs.Service.ExecStart;
      msg = "An absolute mount path is not used as-is: ${absolute.config.systemd.user.services.task-fs.Service.ExecStart}";
    }
    {
      cond = lib.hasInfix "mount --rm delete /home/testuser/some/path" flagged.config.systemd.user.services.task-fs.Service.ExecStart;
      msg = "automount.mountFlags are not passed to the mount command: ${flagged.config.systemd.user.services.task-fs.Service.ExecStart}";
    }
    {
      cond = lib.all (
        start:
        !lib.hasInfix "--view" start
        && !lib.hasInfix "--virtual-tags" start
        && !lib.hasInfix "--virtual-hidden" start
        && !lib.hasInfix "-H " start
      ) [
        unit.Service.ExecStart
        flagged.config.systemd.user.services.task-fs.Service.ExecStart
      ];
      msg = "A mount command advertises a removed view or virtual-tag flag";
    }
    {
      cond =
        lib.hasInfix "XDG_STATE_HOME" default.options.services.task-fs.automount.mountFlags.description
        && lib.hasInfix "_filter" default.options.services.task-fs.automount.mountFlags.description;
      msg = "The mountFlags description does not document runtime views and their state directory";
    }
    {
      cond = disabled.config.home.packages == [ ] && !(disabled.config.systemd.user.services ? task-fs);
      msg = "A disabled module installs the package or creates the service";
    }
    {
      cond = lib.any (a: !a.assertion && lib.hasInfix "Linux" a.message) nonLinux.config.assertions;
      msg = "Non-Linux evaluation does not produce the Linux-only assertion failure";
    }
    {
      cond = wrapped.config.services.task-fs.package == flakePackage;
      msg = "The wrapped homeModules.task-fs output does not default to the flake package";
    }
    {
      cond = lib.hasPrefix (toString flakePackage) wrappedUnit.Service.ExecStart;
      msg = "The wrapped homeModules.task-fs output does not mount with the flake package: ${wrappedUnit.Service.ExecStart}";
    }
  ];

  failures = lib.catAttrs "msg" (lib.filter (c: !c.cond) checks);
in
pkgs.runCommand "task-fs-home-manager-module-tests" { } (
  if failures == [ ] then
    "touch $out"
  else
    throw "task-fs home-manager module checks failed:\n  ${lib.concatStringsSep "\n  " failures}"
)
