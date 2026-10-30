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

  default = eval {
    config.services.task-fs = {
      enable = true;
      mounts = {
        work = {
          path = "~/work-tasks";
        };
        personal = {
          path = "~/personal-tasks";
        };
      };
    };
  };
  defaultUnits = default.config.systemd.user.services;
  workUnit = defaultUnits."task-fs-work";
  personalUnit = defaultUnits."task-fs-personal";

  single = eval {
    config.services.task-fs = {
      enable = true;
      mounts.work.path = "~/work";
    };
  };

  customPkg = pkgs.hello;
  custom = eval {
    config.services.task-fs = {
      enable = true;
      package = customPkg;
      mounts = {
        work.path = "~/work";
        personal.path = "~/personal";
      };
    };
  };
  customUnits = custom.config.systemd.user.services;

  absolute = eval {
    config.services.task-fs = {
      enable = true;
      mounts.tasks.path = "/mnt/tasks";
    };
  };

  flagged = eval {
    config.services.task-fs = {
      enable = true;
      mounts = {
        work = {
          path = "~/work";
          mountFlags = [
            "--rm"
            "delete"
          ];
        };
        personal.path = "~/personal";
      };
    };
  };
  flaggedUnits = flagged.config.systemd.user.services;

  spaced = eval {
    config.services.task-fs = {
      enable = true;
      mounts.work = {
        path = "~/work";
        mountFlags = [
          "--rm"
          "delete"
          "value with spaces"
        ];
      };
    };
  };

  extraPackaged = eval {
    config.services.task-fs = {
      enable = true;
      mounts = {
        work = {
          path = "~/work";
          extraPackages = [ pkgs.hello ];
        };
        personal.path = "~/personal";
      };
    };
  };
  extraPackagedUnits = extraPackaged.config.systemd.user.services;

  taskData = eval {
    config.services.task-fs = {
      enable = true;
      mounts = {
        work = {
          path = "~/work";
          taskData = "/var/lib/task/work";
        };
        personal = {
          path = "~/personal";
          taskData = "~/task-data/personal";
        };
      };
    };
  };
  taskDataUnits = taskData.config.systemd.user.services;

  disabled = eval {
    config.services.task-fs = {
      enable = false;
      mounts.work.path = "~/work";
    };
  };

  invalidName = eval {
    config.services.task-fs = {
      enable = true;
      mounts = {
        "bad/name".path = "~/bad";
      };
    };
  };

  emptyPath = eval {
    config.services.task-fs = {
      enable = true;
      mounts.empty.path = "";
    };
  };

  emptyTaskData = eval {
    config.services.task-fs = {
      enable = true;
      mounts = {
        bad = {
          path = "~/bad";
          taskData = "";
        };
      };
    };
  };

  wrapped = lib.evalModules {
    modules = [
      wrappedModule
      stubs
      {
        config.services.task-fs = {
          enable = true;
          mounts.work.path = "~/work";
        };
      }
    ];
    specialArgs = { inherit pkgs; };
  };
  wrappedUnit = wrapped.config.systemd.user.services."task-fs-work";

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
      mounts.work.path = "~/work";
    };
  };

  envHasPrefix = prefix: unit: lib.any (entry: lib.hasPrefix prefix entry) unit.Service.Environment;

  envEntry =
    prefix: unit: lib.findFirst (entry: lib.hasPrefix prefix entry) null unit.Service.Environment;

  allPass = config: lib.all (a: a.assertion) config.config.assertions;

  hasFailedAssertion =
    config: pattern:
    lib.any (a: !a.assertion && lib.hasInfix pattern a.message) config.config.assertions;

  allExecStarts = map (unit: unit.Service.ExecStart) [
    workUnit
    personalUnit
    customUnits."task-fs-work"
    customUnits."task-fs-personal"
    flaggedUnits."task-fs-work"
    flaggedUnits."task-fs-personal"
    spaced.config.systemd.user.services."task-fs-work"
    extraPackagedUnits."task-fs-work"
    extraPackagedUnits."task-fs-personal"
    taskDataUnits."task-fs-work"
    taskDataUnits."task-fs-personal"
  ];

  checks = [
    {
      cond = default.options.services.task-fs ? mounts && !(default.options.services.task-fs ? automount);
      msg = "The option tree does not expose services.task-fs.mounts or still exposes services.task-fs.automount";
    }
    {
      cond =
        builtins.attrNames defaultUnits == [
          "task-fs-personal"
          "task-fs-work"
        ];
      msg = "A two-mount configuration does not produce exactly task-fs-personal and task-fs-work: ${builtins.toJSON (builtins.attrNames defaultUnits)}";
    }
    {
      cond = builtins.attrNames single.config.systemd.user.services == [ "task-fs-work" ];
      msg = "A one-mount configuration produces unexpected services: ${builtins.toJSON (builtins.attrNames single.config.systemd.user.services)}";
    }
    {
      cond = workUnit.Service.ExecStart == "${flakePackage}/bin/task-fs mount /home/testuser/work-tasks";
      msg = "The work unit does not mount its ~-expanded path: ${workUnit.Service.ExecStart}";
    }
    {
      cond =
        personalUnit.Service.ExecStart == "${flakePackage}/bin/task-fs mount /home/testuser/personal-tasks";
      msg = "The personal unit does not mount its ~-expanded path: ${personalUnit.Service.ExecStart}";
    }
    {
      cond =
        lib.any (
          p:
          lib.hasInfix "mountpoint -q /home/testuser/work-tasks" p
          && lib.hasInfix "/run/wrappers/bin/fusermount3 -u /home/testuser/work-tasks" p
        ) workUnit.Service.ExecStartPre
        && lib.any (p: lib.hasInfix "mkdir -p /home/testuser/work-tasks" p) workUnit.Service.ExecStartPre;
      msg = "The work unit does not clear a stale mount or create its path: ${builtins.toJSON workUnit.Service.ExecStartPre}";
    }
    {
      cond =
        lib.any (
          p:
          lib.hasInfix "mountpoint -q /home/testuser/personal-tasks" p
          && lib.hasInfix "/run/wrappers/bin/fusermount3 -u /home/testuser/personal-tasks" p
        ) personalUnit.Service.ExecStartPre
        && lib.any (
          p: lib.hasInfix "mkdir -p /home/testuser/personal-tasks" p
        ) personalUnit.Service.ExecStartPre;
      msg = "The personal unit does not clear a stale mount or create its path: ${builtins.toJSON personalUnit.Service.ExecStartPre}";
    }
    {
      cond =
        workUnit.Service.ExecStopPost == "-/run/wrappers/bin/fusermount3 -u /home/testuser/work-tasks"
        &&
          personalUnit.Service.ExecStopPost
          == "-/run/wrappers/bin/fusermount3 -u /home/testuser/personal-tasks";
      msg = "A mount unit does not unmount its own path: ${
        builtins.toJSON [
          workUnit.Service.ExecStopPost
          personalUnit.Service.ExecStopPost
        ]
      }";
    }
    {
      cond =
        workUnit.Service.Restart == "on-failure"
        && personalUnit.Service.Restart == "on-failure"
        && workUnit.Install.WantedBy == [ "default.target" ]
        && personalUnit.Install.WantedBy == [ "default.target" ];
      msg = "Mount units do not restart on failure or start with default.target";
    }
    {
      cond = lib.elem flakePackage default.config.home.packages;
      msg = "Enabling does not install the flake package";
    }
    {
      cond =
        allPass default && allPass custom && allPass flagged && allPass extraPackaged && allPass taskData;
      msg = "A valid multi-mount configuration has failing assertions";
    }
    {
      cond =
        lib.hasPrefix (toString customPkg) customUnits."task-fs-work".Service.ExecStart
        && lib.hasPrefix (toString customPkg) customUnits."task-fs-personal".Service.ExecStart
        && lib.elem customPkg custom.config.home.packages;
      msg = "A custom services.task-fs.package does not propagate to every mount unit and the user environment";
    }
    {
      cond =
        lib.hasInfix "mount /mnt/tasks"
          absolute.config.systemd.user.services."task-fs-tasks".Service.ExecStart;
      msg = "An absolute mount path is not used as-is: ${
        absolute.config.systemd.user.services."task-fs-tasks".Service.ExecStart
      }";
    }
    {
      cond =
        lib.hasInfix "mount --rm delete /home/testuser/work" flaggedUnits."task-fs-work".Service.ExecStart
        && !(lib.hasInfix "--rm" flaggedUnits."task-fs-personal".Service.ExecStart);
      msg = "Per-mount mountFlags leak between units: ${
        builtins.toJSON [
          flaggedUnits."task-fs-work".Service.ExecStart
          flaggedUnits."task-fs-personal".Service.ExecStart
        ]
      }";
    }
    {
      cond =
        lib.hasInfix "'value with spaces'"
          spaced.config.systemd.user.services."task-fs-work".Service.ExecStart;
      msg = "A mount flag containing spaces is not quoted as one argument: ${
        spaced.config.systemd.user.services."task-fs-work".Service.ExecStart
      }";
    }
    {
      cond =
        lib.hasInfix "mount /home/testuser/work-tasks" workUnit.Service.ExecStart
        && !(lib.hasInfix "--rm" workUnit.Service.ExecStart);
      msg = "An unset mountFlags list changes the mount command: ${workUnit.Service.ExecStart}";
    }
    {
      cond =
        lib.any (
          entry: lib.hasInfix pkgs.hello.name entry
        ) extraPackagedUnits."task-fs-work".Service.Environment
        && !(lib.any (
          entry: lib.hasInfix pkgs.hello.name entry
        ) extraPackagedUnits."task-fs-personal".Service.Environment);
      msg = "Per-mount extraPackages leak between units: ${
        builtins.toJSON [
          extraPackagedUnits."task-fs-work".Service.Environment
          extraPackagedUnits."task-fs-personal".Service.Environment
        ]
      }";
    }
    {
      cond =
        envHasPrefix "PATH=" workUnit
        &&
          envEntry "PATH=" workUnit == "PATH=/run/wrappers/bin:${
            lib.makeBinPath [
              pkgs.taskwarrior3
              pkgs.fuse3
              pkgs.coreutils
            ]
          }";
      msg = "A mount without extraPackages does not get exactly the baseline PATH: ${builtins.toJSON workUnit.Service.Environment}";
    }
    {
      cond =
        envEntry "TASKDATA=" taskDataUnits."task-fs-work" == "TASKDATA=/var/lib/task/work"
        &&
          envEntry "TASKDATA=" taskDataUnits."task-fs-personal"
          == "TASKDATA=/home/testuser/task-data/personal";
      msg = "taskData is not resolved into TASKDATA per mount: ${
        builtins.toJSON [
          taskDataUnits."task-fs-work".Service.Environment
          taskDataUnits."task-fs-personal".Service.Environment
        ]
      }";
    }
    {
      cond = !(envHasPrefix "TASKDATA=" workUnit) && !(envHasPrefix "TASKDATA=" personalUnit);
      msg = "A mount without taskData unexpectedly sets TASKDATA";
    }
    {
      cond =
        disabled.config.home.packages == [ ]
        && disabled.config.systemd.user.services == { }
        && disabled.config.assertions == [ ];
      msg = "A disabled module installs a package, creates a unit, or asserts";
    }
    {
      cond =
        wrapped.config.services.task-fs.package == flakePackage
        && lib.hasPrefix (toString flakePackage) wrappedUnit.Service.ExecStart;
      msg = "The wrapped homeModules.task-fs output does not default to the flake package for named mounts";
    }
    {
      cond = hasFailedAssertion nonLinux "Linux";
      msg = "Non-Linux evaluation does not produce the Linux-only assertion failure";
    }
    {
      cond = hasFailedAssertion invalidName "letters, digits, hyphens, and underscores";
      msg = "An invalid mount name does not produce the naming assertion: ${builtins.toJSON invalidName.config.assertions}";
    }
    {
      cond = hasFailedAssertion emptyPath "must not be empty";
      msg = "An empty mount path does not produce the path assertion: ${builtins.toJSON emptyPath.config.assertions}";
    }
    {
      cond = hasFailedAssertion emptyTaskData "taskData must not be empty";
      msg = "An empty taskData does not produce the taskData assertion: ${builtins.toJSON emptyTaskData.config.assertions}";
    }
    {
      cond = lib.all (
        start:
        !lib.hasInfix "--view" start
        && !lib.hasInfix "--virtual-tags" start
        && !lib.hasInfix "--virtual-hidden" start
        && !lib.hasInfix "-H " start
      ) allExecStarts;
      msg = "A mount command advertises a removed view or virtual-tag flag";
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
