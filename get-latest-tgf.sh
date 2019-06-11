#!/usr/bin/env bash

TGF=${TGF_PATH:=/usr/local/bin}/tgf

if [ ! -d "$TGF_PATH" ]; then
  echo "creating "$TGF_PATH" directory..."
  mkdir -p $TGF_PATH
fi

get_local_tgf_version () {
    if [ -z "$TGF_LOCAL_VERSION" ]; then
        TGF_LOCAL_VERSION=$($TGF --current-version | awk -F\  '{print $2}')
    else
        echo "TGF_LOCAL_VERSION set to" $TGF_LOCAL_VERSION
    fi
}

get_latest_tgf_version () {
    if [ -z "$GITHUB_TOKEN" ]
    then
        echo 'GITHUB_TOKEN is not set.'
    fi    
    
    TGF_LATEST_VERSION=$(curl --silent https://api.github.com/repos/coveo/tgf/releases/latest?access_token=${GITHUB_TOKEN} | grep tag_name | awk -F\" '{print $4}')
    
    if [ -z "$TGF_LATEST_VERSION" ]
    then 
        echo "Could not obtain tgf latest version. (check your GITHUB_TOKEN)"
        exit 1
    fi
}

script_end () {
    echo 'Done.'
    $TGF --current-version
    exit 0
}

install_latest_tgf () {
    VERSION=$(echo $TGF_LATEST_VERSION | cut -d'v' -f 2)

    if [[ $(uname -s) == Linux ]]
    then
        echo 'Installing latest tgf version for Linux in' $TGF_PATH '...'
        curl -sL "https://github.com/coveo/tgf/releases/download/v"$VERSION"/tgf_"$VERSION"_linux_64-bits.zip" | gzip -d > $TGF && chmod +x $TGF && script_end
    elif [[ $(uname -s) == Darwin ]]
    then
        echo 'Installing latest tgf for OSX in' $TGF_PATH '...'
        curl -sL "https://github.com/coveo/tgf/releases/download/v"$VERSION"/tgf_"$VERSION"_macOS_64-bits.zip" | bsdtar -xf- -C $TGF_PATH && script_end
    else 
        echo 'OS not supported.'
        exit 1
    fi
}

get_local_tgf_version
get_latest_tgf_version

echo '- tgf version (local):' $TGF_LOCAL_VERSION
echo '- tgf version (latest):' $TGF_LATEST_VERSION

if [[ $TGF_LOCAL_VERSION == $TGF_LATEST_VERSION ]]
then 
    echo 'Local version is up to date.'
else
    install_latest_tgf
fi
