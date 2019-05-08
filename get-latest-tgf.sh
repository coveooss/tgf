#!/usr/bin/env bash
get_local_tgf_version () {
        TGF_LOCAL_VERSION=$(tgf --current-version | awk -F\  '{print $2}');
}

get_latest_tgf_version () {
    if [ -z "$GITHUB_TOKEN" ]; then
        echo 'Warning: GITHUB_TOKEN is not set.';
    fi
    TGF_LATEST_VERSION=$(curl --silent https://api.github.com/repos/coveo/tgf/releases/latest?access_token=${GITHUB_TOKEN} | grep tag_name | awk -F\" '{print $4}');
}

install_latest_tgf () {
        curl -sL https://github.com/coveo/tgf/releases/download/v1.20.2/tgf_1.20.2_linux_64-bits.zip | gzip -d > /usr/local/bin/tgf && chmod +x /usr/local/bin/tgf;
}

if ! [ -x "$(command -v tgf)" ]; then
  echo 'Error: tgf is not installed. Installing ...';
  install_latest_tgf;
  echo 'Info: Done.'
  tgf --current-version;
  exit 1;
fi

get_local_tgf_version;
get_latest_tgf_version;

echo '- tgf version (local):' $TGF_LOCAL_VERSION;
echo '- tgf version (latest):' $TGF_LATEST_VERSION;

if [ "$TGF_LOCAL_VERSION" == "$TGF_LATEST_VERSION" ]
then echo 'Info: local version is up to date';
else
        echo 'Warning: local version outdated, updating...';
        install_latest_tgf;
        get_local_tgf_version;
        echo 'Info: tgf updated to' $TGF_LOCAL_VERSION;
fi
