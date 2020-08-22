# builder image
FROM surnet/alpine-wkhtmltopdf:3.10-0.12.6-full as builder

# Image
FROM golang:1.11-alpine3.10

# Install needed packages
RUN  echo "https://mirror.tuna.tsinghua.edu.cn/alpine/v3.10/main" > /etc/apk/repositories \
     && echo "https://mirror.tuna.tsinghua.edu.cn/alpine/v3.10/community" >> /etc/apk/repositories \
     && apk update && apk add --no-cache \
      libstdc++ \
      libx11 \
      libxrender \
      libxext \
      libssl1.1 \
      ca-certificates \
      fontconfig \
      freetype \
      ttf-dejavu \
      ttf-droid \
      ttf-freefont \
      ttf-liberation \
      ttf-ubuntu-font-family \
    && apk add --no-cache --virtual .build-deps \
      msttcorefonts-installer \
    \
    # Install microsoft fonts
    && update-ms-fonts \
    && fc-cache -f \
    \
    # Clean up when done
    && rm -rf /var/cache/apk/* \
    && rm -rf /tmp/* \
    && apk del .build-deps

COPY --from=builder /bin/wkhtmltopdf /bin/wkhtmltopdf
COPY --from=builder /bin/wkhtmltoimage /bin/wkhtmltoimage

WORKDIR /go/src/app

COPY fonts/* /usr/share/fonts

COPY src/* .

RUN go get -d -v ./... && go install -v ./...

EXPOSE 80

CMD [ "app" ]
