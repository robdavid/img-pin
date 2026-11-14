{
  description = "img-pin package and development shell";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    nixpkgs-2605.url = "github:NixOS/nixpkgs/nixos-26.05";
  };

  outputs = { self, nixpkgs, nixpkgs-2605 }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          stable = nixpkgs-2605.legacyPackages.${system};
        in
        {
          default = stable.buildGo127Module {
            pname = "img-pin";
            version = "0.5.2";
            src = self;
            vendorHash = "sha256-7jzu6/GNvuug6OExnOAjTfHMv3Mem5OqFZuOhAecB34=";
            checkFlags = [ "-short" ];
          };
        }
      );
      devShells = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          stable = nixpkgs-2605.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              stable.go_1_27
              gopls
              delve
              govulncheck
              kubernetes-helm
              docker-client
              bashInteractive # https://discourse.nixos.org/t/interactive-bash-with-nix-develop-flake/15486
              nixd
              nil
            ];
            hardeningDisable = [ "fortify" ];
          };
        }
      );
    };
}
