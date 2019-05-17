FROM golang:1.12-stretch

RUN echo "deb http://packages.cloud.google.com/apt cloud-sdk-stretch main" > /etc/apt/sources.list.d/google-cloud-sdk.list && \
    curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - && \
    apt-get update -y && apt-get install -y \
        google-cloud-sdk \
        vim

ENV os=linux
ENV arch=amd64  

RUN curl -sL https://go.kubebuilder.io/dl/2.0.0-alpha.1/${os}/${arch} | tar -xz -C /tmp/ && \
    mv /tmp/kubebuilder_2.0.0-alpha.1_${os}_${arch} /usr/local/kubebuilder && \
    curl -o /usr/local/kubebuilder/bin/kustomize -sL https://go.kubebuilder.io/kustomize/${os}/${arch} && \
    chmod +x /usr/local/kubebuilder/bin/kustomize

ENV PATH=$PATH:/usr/local/kubebuilder/bin

ENV GO111MODULE=on

RUN go get sigs.k8s.io/controller-runtime@v0.2.0-beta.1 && \
    go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.0-beta.1

CMD ["/bin/bash"]
