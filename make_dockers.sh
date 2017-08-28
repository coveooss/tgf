set -e

# If TRAVIS_TAG is not set, we recuperate the last tag for the repo
: ${TRAVIS_TAG:=$(git describe --abbrev=0 --tags)}

# If the tag does not start with image- we ignore it
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
    
    dockerfile=dockerfile.temp
    # We replace the TRAVIS_TAG variable if any (case where the image is build from another image)
    # The result file is simply named Dockerfile
    cat $df | sed -e "s/\${TRAVIS_TAG}/$travis_tag/" > $dockerfile

    docker build -f $dockerfile -t $version . && rm $dockerfile
    docker push $version
    if [ -n "$latest" ]
    then 
        docker tag $version $latest && docker push $latest
    fi
done
