echo "using src $src"
cd $src

export GOTOOLCHAIN=local
export GOCACHE=$TMPDIR/.gocache
export GOPATH=${splitmodsjoin:-$TMPDIR/go}

echo "getting go build -n output..."
build=$TMPDIR/build-n
$go/bin/go build -n ${subPackage:+./${subPackage}} 2> $build
ret=$?
if [[ $ret -ne 0 ]]; then
  cat $build >&2
  exit ret
fi

echo "generating nix..."
dgdnix=$TMPDIR/dgd.nix
$processBuild/bin/process-build \
  -script "$TMPDIR/build-n" \
  -rootdir "$src" \
  -deproot "$splitmodsjoin" \
  > $dgdnix

if [[ $out == *.nix ]]; then
  cp $dgdnix $out
  exit 0
fi

echo "instantiating drv..."
drv=$($innerNix/bin/nix-instantiate \
  --argstr srcStr "$src" \
  --argstr goStr "$go" \
  --argstr envJson "$envJson" \
  --argstr pkgsPath "$pkgsPath" \
  $dgdnix)
cp $drv $out
