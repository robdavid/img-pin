{
  description = "img-pin package and development shell";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.buildGoModule {
            pname = "img-pin";
            version = "0.3.1";
            src = self;
            vendorHash = "sha256-mLyeM7BxRlhT6LlNU0haE41ss4MXFhUO9T5c1b8cULU=";
            checkFlags = [ "-short" ];
          };
        }
      );
      devShells = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              go
              gopls
              delve
              govulncheck
              kubernetes-helm
              docker-client
              bashInteractive # https://discourse.nixos.org/t/interactive-bash-with-nix-develop-flake/15486
            ];
            hardeningDisable = [ "fortify" ];
          };
        }
      );
    };
}
