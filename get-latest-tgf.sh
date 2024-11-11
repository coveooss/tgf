#!/usr/bin/env bash

DEFAULT_INSTALL_DIR=/usr/local/bin
TGF=${TGF_PATH:=${DEFAULT_INSTALL_DIR}}/tgf

if [ ! -d "${TGF_PATH}" ]; then
  echo "creating ${TGF_PATH} directory..."
  mkdir -p "${TGF_PATH}"
fi

if [ ! -w "${TGF_PATH}" ]; then
  if [ "${TGF_PATH}" == "${DEFAULT_INSTALL_DIR}" ]; then
    # This is a system directory.  It's not a good idea to try to make it writeable.
    echo "System installation directory ${TGF_PATH} is not writeable."
    if [[ "$OSTYPE" == "darwin"* && "${TGF_PATH}" == "${DEFAULT_INSTALL_DIR}" ]]; then
        # Mac OSX
        echo "Since MacOS Ventura (13.X), $TGF_PATH is owned by 'root'."
        echo "You can fix this by either"
        echo "  1. Running 'sudo chown -R $(id -un):$(id -gn) $TGF_PATH'."
        echo "  2. Setting the TGF_PATH (GitHub action's parameter 'tgf-path') variable to an alternate, writable directory."
        exit 1
    fi
    echo "Please set the TGF_PATH (GitHub action's parameter 'tgf-path') environment variable to install to an alternate, writeable directory."
    exit 1
  fi


  if ! chmod ugo+w "${TGF_PATH}"; then
    echo "Cannot make installation directory ${TGF_PATH} writeable."
    echo "Please set the TGF_PATH (GitHub action's parameter 'tgf-path') environment variable to install to an alternate, writeable directory."
    exit 1
  fi
fi

get_local_tgf_version () {
    # The sed regex extracts for example 1.23.2 from "tgf v1.23.2", so as to be comparable to get_latest_tgf_version()
    [ -r "${TGF}" ] && TGF_LOCAL_VERSION=$(${TGF} --help-tgf | grep 'VERSION: ' | sed 's/VERSION: //')
}

get_latest_tgf_version () {
    # The github releases/latest page redirects you to the latest release. We extract the version from the url of the
    # `location` header. We could have used the API here but this avoids rate limits. It's also easier to parse http
    # headers in bash than it is to parse JSON.
    TGF_LATEST_VERSION="$(
        curl --fail --silent --show-error --head https://github.com/coveooss/tgf/releases/latest \
            | grep -i '^location: ' \
            | cut -d ' ' -f 2 \
            | cut -d '/' -f 8 \
            | tr -d '[:space:]' \
            | sed 's/^v//'
    )"

    if [ -z "$TGF_LATEST_VERSION" ]
    then
        echo "Could not obtain tgf latest version."
        exit 1
    fi
}

script_end () {
    echo 'Done.'
    ${TGF} --current-version
    exit 0
}

install_latest_tgf () {
    if [[ $(uname -s) == Linux ]]
    then
        LINUX_ARCH=$(uname -m)
        echo 'Installing latest tgf for Linux with arch '$LINUX_ARCH' in' $TGF_PATH '...'
        DOWNLOAD_URL=$([ "$LINUX_ARCH" == "x86_64" ] && echo "https://github.com/coveooss/tgf/releases/download/v${TGF_LATEST_VERSION}/tgf_${TGF_LATEST_VERSION}_linux_64-bits.zip" || echo "https://github.com/coveooss/tgf/releases/download/v${TGF_LATEST_VERSION}/tgf_${TGF_LATEST_VERSION}_linux_arm64.zip")
        curl -sL $DOWNLOAD_URL | gzip -d > ${TGF} && chmod 0755 ${TGF} && script_end
    elif [[ $(uname -s) == Darwin ]]
    then
        OSX_ARCH=$(uname -m)
        echo 'Installing latest tgf for OSX with arch '$OSX_ARCH' in' $TGF_PATH '...'
        DOWNLOAD_URL=$([ "$OSX_ARCH" == "arm64" ] && echo "https://github.com/coveooss/tgf/releases/download/v${TGF_LATEST_VERSION}/tgf_${TGF_LATEST_VERSION}_macOS_arm64.zip" || echo "https://github.com/coveooss/tgf/releases/download/v${TGF_LATEST_VERSION}/tgf_${TGF_LATEST_VERSION}_macOS_64-bits.zip")
        curl -sL $DOWNLOAD_URL | bsdtar -xf- -C ${TGF_PATH} && chmod 0755 ${TGF} && script_end
    else
        echo 'OS not supported.'
        exit 1
    fi
}

get_local_tgf_version
get_latest_tgf_version

echo '- tgf version (local) :' "${TGF_LOCAL_VERSION}"
echo '- tgf version (latest):' "${TGF_LATEST_VERSION}"

if [[ "${TGF_LOCAL_VERSION}" == "${TGF_LATEST_VERSION}" ]]
then
    echo 'Local version is up to date.'
else
    install_latest_tgf
fi
