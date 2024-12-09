# ocrmypdf-watchdog

This is a simple watchdog for OCRMyPDF (and maybe others). It watches a given folder for new files with definable extensions and runs then ocrmypdf (or another command) to convert files to pdf.

## Docker

The Dockerfile creates an image based on the jbarlow83/ocrmypdf image and adds the watchdog.

    docker-compose up -d
 
 There are 3 volumes: <b>/in</b>, <b>/bak</b> and <b>/out</b>
 The docker-compose.yml shows how to use them.
 
 ## Environment
 
The watchdog looks for the following environment variables:
 
* OCRMYPDF_IN
* OCRMYPDF_BAK
* OCRMYPDF_OUT
* OCRMYPDF_BINARY
* OCRMYPDF_PARAMETER
* WATCHDOG_EXTENSIONS
* WATCHDOG_FREQUENCY

## Parameters

The watchdog accepts the following parameters:

* --in <in-path>
* --bak <backup-path>
* --out <out-path>
* --frequency <in seconds>
* --ocrmypdf <path and name of the executable>

## Example compose.yml
    services:
      ocrmypdf-watchdog:
        container_name: ocrmypdf-watchdog
        environment:
          OCRMYPDF_BAK: /bak
          OCRMYPDF_BINARY: ocrmypdf
          OCRMYPDF_IN: /in
          OCRMYPDF_OUT: /out
          OCRMYPDF_PARAMETER: '-l deu+eng+fra --rotate-pages --deskew --clean --optimize 3 --jobs 4'
          WATCHDOG_EXTENSIONS: pdf,jpg,jpeg,tif,tiff,png,gif
          WATCHDOG_FREQUENCY: 60
        image: unze/ocrmypdf-watchdog:latest
        network_mode: none
        restart: always
        volumes:
          - /host/path/input:/in
          - /host/path/backup:/bak
          - /host/path/output:/out
