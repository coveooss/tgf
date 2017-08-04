[ "${TRAVIS_TAG::6}" != image- ] && exit 0

for df in Dockerfile*
do
    echo
    printf '%40s\n' | tr ' ' -
    printf "Processing file $df\n"
    tag=${df:12}
    tag=${tag,,}
    travis_tag=${TRAVIS_TAG:6}
    version=coveo/tgf:${travis_tag}${tag}
    latest=${tag:1}
    latest=coveo/tgf:${latest:-latest}

    # We do not want to tag latest if this is not an official version number
    [[ $travis_tag == *-* ]] && unset latest
    
    docker build -f $df -t $version . && docker push $version &&
    [ -n "$latest" ] && docker tag $version $latest && docker push $latest
done
