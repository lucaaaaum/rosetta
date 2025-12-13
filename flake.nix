{
  description = "rosetta: where three languages meet to fix a dumb .NET/Nix compatibility problem";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixpkgs-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      version = "1.0.0";
      supportedSystems = [
        "x86_64-linux"
        "x86_64-darwin"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          default = pkgs.buildGoModule {
            pname = "rosetta";
            inherit version;
            src = ./.;
            vendorHash = null;
            nativeBuildInput = with pkgs; [
              go
              gopls
              gotools
              go-tools
              dotnet-sdk_10
            ];
            buildInputs = with pkgs; [
              dotnet-runtime_10
            ];
          };
        }
      );

      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [
              go
              gopls
              gotools
              go-tools
              dotnet-sdk_10
            ];
          };
        }
      );
    };
}
