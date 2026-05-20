{
  description = "code-agent — code review MCP server";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" ];
      forAll = nixpkgs.lib.genAttrs systems;
      pkgsFor = s: nixpkgs.legacyPackages.${s};
    in {
      packages = forAll (s: {
        default = (pkgsFor s).buildGoModule {
          pname = "code-agent";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-478T/7A5V94DPG8Mldbp/Uc28wlZeWe46q61uzJnOZM=";
          subPackages = [ "cmd/code-agent" ];
          # mattn/go-sqlite3 needs cgo (enabled by default in buildGoModule).
        };
      });
      overlays.default = final: prev: {
        code-agent = self.packages.${final.system}.default;
      };
    };
}
