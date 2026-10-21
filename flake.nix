{
  description = "TaskFS Flake";

  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } (
      top@{
        config,
        lib,
        self,
        ...
      }:
      {
        imports = [
        ];
        systems = [
          "x86_64-linux"
          "aarch64-linux"
          "aarch64-darwin"
        ];
        perSystem =
          {
            config,
            self',
            inputs',
            pkgs,
            system,
            ...
          }:
          {
            packages.default = pkgs.buildGoModule {
              pname = "task-fs";
              version = "0.1.0";
              src = ./.;
              vendorHash = "sha256-DLY5UEeBZN/fz+qDZ+Nni9DyzIayMP5rAjQJPyK1yQc=";
            };

            checks.hm-module = import ./nix/hm-module-test.nix {
              lib = pkgs.lib;
              inherit pkgs;
              module = import ./nix/home-manager.nix;
              wrappedModule = self.homeModules.task-fs;
              flakePackage = self'.packages.default;
              taskFsFlake = {
                packages.${system}.default = self'.packages.default;
              };
            };

            devShells.default = pkgs.mkShell {
              packages = with pkgs; [
                openspec
                go
                fuse3
                taskwarrior3
              ];
            };
          };
        flake = {
          homeModules.task-fs = {
            imports = [ ./nix/home-manager.nix ];
            _module.args.taskFsFlake = inputs.self;
          };
        };
      }
    );
}
