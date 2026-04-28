{ pkgs, ... }: {
  packages = with pkgs; [ git gofumpt sqlite golangci-lint goose gore ];
  languages = {
    go.enable = true;
    nix.enable = true;
  };
}
