#!/bin/sh

inputFile=$1
skip=$2

FPS=12
WIDTH=960

ffprobe -v error -count_frames -select_streams v:0 -show_entries stream=nb_read_frames -of default=nokey=1:noprint_wrappers=1 $inputFile

#Generate palette for better quality
ffmpeg -ss $skip -t 35 -i $inputFile -vf fps=$FPS,scale=$WIDTH:-1:flags=lanczos,palettegen /tmp/tmp_palette.png

#Generate gif using palette
rm output.gif
ffmpeg -ss $skip -t 35 -i $inputFile -i /tmp/tmp_palette.png -filter_complex "fade=out:430:5,crop=1600:1400:0:112,fps=$FPS,scale=$WIDTH:-1:flags=lanczos[x];[x][1:v]paletteuse" output.gif

rm /tmp/tmp_palette.png

