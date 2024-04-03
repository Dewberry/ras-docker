# FROM registry.access.redhat.com/ubi8/ubi:8.5 as builder  # image URL stopped working Spring 2024
FROM redhat/ubi8:8.5 as builder
RUN yum -y update && \
    yum -y install wget && \
    yum -y install unzip && \
    yum -y install git

RUN wget https://www.hec.usace.army.mil/software/hec-ras/downloads/HEC-RAS_610_Linux.zip

RUN unzip HEC-RAS_610_Linux.zip
RUN unzip HEC-RAS_610_Linux/RAS_Linux_test_setup.zip
RUN mkdir -p /ras/libs
RUN mv /RAS_Linux_test_setup/libs /ras
RUN mv /RAS_Linux_test_setup/Ras_v61/Release /ras/v61
RUN chmod +x ras/v61/*

ENV GOROOT=/go
ENV GOPATH=/src/go
ENV PATH=/go/bin:$PATH
RUN wget https://golang.org/dl/go1.19.1.linux-amd64.tar.gz -P / && \
    tar -xvzf /go1.19.1.linux-amd64.tar.gz -C /

RUN mkdir -p  /sim/model 
COPY ./ /app
WORKDIR /app

RUN chmod +x run-model.sh
RUN go build main.go

# FROM registry.access.redhat.com/ubi8/ubi:8.5 as prod    # image URL stopped working Spring 2024
FROM redhat/ubi8:8.5 as prod
RUN mkdir -p  /sim/model
RUN mkdir -p app/
COPY --from=builder /app/main /app
COPY --from=builder /app/run-model.sh /app
COPY --from=builder /ras/libs /ras/libs
COPY --from=builder /ras/v61/RasUnsteady /ras/v61/RasUnsteady
RUN chmod +x /ras/v61/*
WORKDIR /app
