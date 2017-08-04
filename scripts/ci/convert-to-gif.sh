#!/bin/sh

inputFile=$1
outputFile=$2

FPS=12
WIDTH=960

# cleanup previous
rm -f /tmp/tmp_palette.png output.gif

ffprobe -v error -count_frames -select_streams v:0 -show_entries stream=nb_read_frames -of default=nokey=1:noprint_wrappers=1 $inputFile

#Generate palette for better quality
ffmpeg -i $inputFile -vf fps=$FPS,scale=$WIDTH:-1:flags=lanczos,palettegen /tmp/tmp_palette.png

#Generate gif using palette
duration=$( ffprobe -i vid_chrome_25550_firefox_25551.mp4 -show_entries stream=codec_type,duration -of compact=p=0:nk=1 | grep video | cut -d '|' -f 2 | cut -d '.' -f 1 )
fade_out=1
offset=$(( $duration - $fade_out ))
ffmpeg -i $inputFile -i /tmp/tmp_palette.png -filter_complex "fade=t=out:st=$offset:d=$fade_out,crop=1600:900:0:70,fps=$FPS,scale=$WIDTH:-1:flags=lanczos[x];[x][1:v]paletteuse" $outputFile
