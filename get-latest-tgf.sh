#!/usr/bin/env bash
get_local_tgf_version () {
        TGF_LOCAL_VERSION=$(tgf --current-version | awk -F\  '{print $2}')
}

get_latest_tgf_version () {
    if [ -z "$GITHUB_TOKEN" ]
    then
        echo 'GITHUB_TOKEN is not set.';
    fi
    TGF_LATEST_VERSION=$(curl --silent https://api.github.com/repos/coveo/tgf/releases/latest?access_token=${GITHUB_TOKEN} | grep tag_name | awk -F\" '{print $4}')
}

done_script () {
    echo "Done.";
    tgf --current-version;
    exit 0;
}

install_latest_tgf () {
        VERSION=$(echo $TGF_LATEST_VERSION | cut -d'v' -f 2);
        
        if [[ "$OSTYPE" == "linux"* ]]; then
                echo 'Installing latest tgf for Linux...';
                curl -sL "https://github.com/coveo/tgf/releases/download/v"$VERSION"/tgf_"$VERSION"_linux_64-bits.zip" | gzip -d > /usr/local/bin/tgf && chmod +x /usr/local/bin/tgf && done_script;
        elif [[ "$OSTYPE" == "darwin"* ]]; then
                echo 'Installing latest tgf for OSX...';
                curl -sL "https://github.com/coveo/tgf/releases/download/v"$VERSION"/tgf_"$VERSION"_macOS_64-bits.zip" | bsdtar -xf- -C /usr/local/bin && done_script;
        else 
                echo 'OS not supported.';
                exit 1;
        fi
}

get_local_tgf_version
get_latest_tgf_version

echo '- tgf version (local):' $TGF_LOCAL_VERSION
echo '- tgf version (latest):' $TGF_LATEST_VERSION;

if [ "$TGF_LOCAL_VERSION" == "$TGF_LATEST_VERSION" ]; then 
        echo 'Local version is up to date.';
else
        install_latest_tgf
fi
