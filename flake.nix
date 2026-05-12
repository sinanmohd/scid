{
  inputs.nixpkgs.url = "github:NixOs/nixpkgs/nixos-unstable";

  outputs =
    inputs@{ self, nixpkgs }:
    let
      lib = nixpkgs.lib;

      forSystem =
        f: system:
        f {
          inherit system;
          pkgs = import nixpkgs { inherit system; };
        };
      supportedSystems = lib.platforms.unix;
      forAllSystems = f: lib.genAttrs supportedSystems (forSystem f);
    in
    {
      packages = forAllSystems (
        { system, pkgs }:
        {
          scid = pkgs.callPackage ./nix/package.nix { };
          default = self.packages.${system}.scid;
          oci = pkgs.callPackage ./nix/oci.nix {
            scid = self.packages.${system}.scid;
          };
        }
      );

      nixosModules = {
        scid = import ./nix/module.nix inputs;
        default = self.nixosModules.scid;
      };

      devShells = forAllSystems (
        { system, pkgs }:
        {
          scid = pkgs.callPackage ./nix/shell.nix {
            scid = self.packages.${system}.scid;
          };
          default = self.devShells.${system}.scid;
        }
      );
    };
}
