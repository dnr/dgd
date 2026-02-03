echo "using src $src"
cd $src
export GOTOOLCHAIN=local
export GOPATH=$out
echo "getting deps..."
$go/bin/go mod download -x
# The Go module cache contains 'pkg/mod/cache' that mirrors that mirrors the
# module proxy structure, including zip files, and then expanded code under
# 'pkg/mod/<module>'. We want to include the expanded code so the build can
# refer to it directly without unzipping/copying, but retain minimal other
# data. We can't delete 'cache' entirely since Go needs it to find the code.
# But we can delete the zip files.
find $GOPATH/pkg/mod/cache \( -name \*.zip -o -name \*.lock -o -name list \) -delete
