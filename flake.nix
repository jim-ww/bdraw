{
  description = "bdraw - a mouse-first paint program for the terminal";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "bdraw";
          version = "0.1.0";
          src = ./.;

          vendorHash = "sha256-UsvPBIPO8cLVmFbZUlueAuiMd5fEGSi6Omf6jTr64Wo=";

          doCheck = false;

          meta = with pkgs.lib; {
            description = "A mouse-first paint program for the terminal";
            homepage = "https://github.com/jim-ww/bdraw";
            license = licenses.gpl3Only;
            mainProgram = "bdraw";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [ pkgs.go ];
        };
      });
}
