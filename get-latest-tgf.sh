#!/usr/bin/env bash

DEFAULT_INSTALL_DIR=/usr/local/bin
TGF=${TGF_PATH:=${DEFAULT_INSTALL_DIR}}/tgf

if [ ! -d "${TGF_PATH}" ]; then
  echo "creating ${TGF_PATH} directory..."
  mkdir -p ${TGF_PATH}
fi

if [ ! -w "${TGF_PATH}" ]; then
  if [ "${TGF_PATH}" == "${DEFAULT_INSTALL_DIR}" ]; then
    # This is a system directory.  It's not a good idea to try to make it writeable.
    echo "System installation directory ${TGF_PATH} is not writeable."
    echo "Please set the TGF_PATH environment variable to install to an alternate, writeable directory."
    exit 1
  fi


  if ! chmod ugo+w "${TGF_PATH}"; then
    echo "Cannot make installation directory ${TGF_PATH} writeable."
    echo "Please set the TGF_PATH environment variable to install to an alternate, writeable directory."
    exit 1
  fi
fi

get_local_tgf_version () {
    # The sed regex extracts for example 1.23.2 from "tgf v1.23.2", so as to be comparable to get_latest_tgf_version()
    [ -r ${TGF} ] && TGF_LOCAL_VERSION=$(${TGF} --current-version | sed -E -e 's/^.* v(.*)/\1/')
}

get_latest_tgf_version () {
    TGF_LATEST_VERSION=$(curl --silent https://coveo-bootstrap-us-east-1.s3.amazonaws.com/tgf_version.txt)
    
    if [ -z "${TGF_LATEST_VERSION}" ]
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
        echo 'Installing latest tgf version for Linux in' $TGF_PATH '...'
        curl -sL "https://github.com/coveooss/tgf/releases/download/v${TGF_LATEST_VERSION}/tgf_${TGF_LATEST_VERSION}_linux_64-bits.zip" | gzip -d > ${TGF} && chmod +x ${TGF} && script_end
    elif [[ $(uname -s) == Darwin ]]
    then
        echo 'Installing latest tgf for OSX in' $TGF_PATH '...'
        curl -sL "https://github.com/coveooss/tgf/releases/download/v${TGF_LATEST_VERSION}/tgf_${TGF_LATEST_VERSION}_macOS_64-bits.zip" | bsdtar -xf- -C ${TGF_PATH} && chmod +x ${TGF} && script_end
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
