{
  description = "Publish CNAME records for the local host over mDNS using Avahi";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = "2.4.0";
      in
      {
        packages = {
          default = pkgs.buildGoModule {
            pname = "go-avahi-cname";
            inherit version;

            src = ./.;

            vendorHash = "sha256-Q7/EH/o1q7HQo81nMI7lIKgJ0OOo257OvuAIphSUZVI=";

            ldflags = [
              "-s"
              "-w"
              "-X main.version=${version}"
              "-X main.commit=${self.shortRev or "dirty"}"
              "-X main.date=1970-01-01T00:00:00Z"
            ];

            meta = {
              description = "Publish CNAME records pointing to local host over mDNS via Avahi";
              homepage = "https://github.com/grishy/go-avahi-cname";
              changelog = "https://github.com/grishy/go-avahi-cname/releases/tag/v${version}";
              license = pkgs.lib.licenses.mit;
              # TODO: uncomment after https://github.com/NixOS/nixpkgs/pull/480646 is merged
              # maintainers = with pkgs.lib.maintainers; [ grishy ];
              maintainers = [ ];
              platforms = pkgs.lib.platforms.unix;
              mainProgram = "go-avahi-cname";
            };
          };

          go-avahi-cname = self.packages.${system}.default;
        };

        devShells.default = pkgs.mkShell {
          inputsFrom = [ self.packages.${system}.default ];
          packages = with pkgs; [
            golangci-lint
            goreleaser
            nixfmt
          ];
        };

        formatter = pkgs.nixfmt;
      }
    );
}
