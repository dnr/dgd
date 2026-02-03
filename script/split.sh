echo "outputting split derivations from $modpkg"
cd $modpkg

declare -a joincmds
next=0

split() {
  path=${1#./} tmp=$(mktemp -d)

  tar -c -C "$path" . | tar -x -C "$tmp"

  h=$($innerNix/bin/nix-hash --type sha256 --sri "$tmp")

  cleanpath=${path#pkg/mod/}
  cleanname=$(echo -n "$cleanpath" | tr -c 'a-zA-Z0-9_.-' '-')
  name="part$((next++))"

  echo "$name = pkgs.runCommandLocal
    \"dgd-split-$cleanname\"
    { outputHash = \"$h\"; outputHashMode = \"recursive\"; }
    \"mkdir \$out && tar -c -C \${modpkg}/$path . | tar -x -C \$out\";"
  joincmds+=(
    "mkdir -p \$out/${path%/*}"
    "ln -s \${${name}} \$out/$path"
    )
}

echo "generating nix..."
splitnix=$TMPDIR/split.nix
{
  if [[ $out == *.nix ]]; then
    echo '{ pkgs, modpkg }: let'
  else
    echo '{ pkgsPath, modpkgStr }: let
      pkgs = import pkgsPath { };
      modpkg = builtins.storePath modpkgStr;'
  fi
  for dir in $(find . -path ./pkg/mod/cache -prune -o -type d -name '*@v*' -print); do
    split "$dir"
  done
  split "./pkg/mod/cache"
  # note: name "dgd-joined-deps" must match go code
  echo 'in pkgs.runCommandLocal "dgd-joined-deps" { }'
  echo "''"
  for j in "${joincmds[@]}"; do
    echo "$j"
  done
  echo "''"
} > $splitnix

if [[ $out == *.nix ]]; then
  cp $splitnix $out
  exit 0
fi

echo "instantiating drv..."
drv=$($innerNix/bin/nix-instantiate \
  --argstr pkgsPath "$pkgsPath" \
  --argstr modpkgStr "$modpkg" \
  $splitnix)
cp $drv $out
