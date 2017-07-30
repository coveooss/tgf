[ "${TRAVIS_TAG::6}" != image- ] && exit 0

for df in Dockerfile*
do
    echo
    printf '%40s\n' | tr ' ' -
    printf "Processing file $df\n"
    tag=${df:12}
    tag=${tag,,}
    version=coveo/tgf:${TRAVIS_TAG:6}${tag}
    latest=${tag:1}
    latest=coveo/tgf:${latest:-latest}
    docker build -f $df -t $version . && docker push $version && docker tag $version $latest && docker push $latest
done
