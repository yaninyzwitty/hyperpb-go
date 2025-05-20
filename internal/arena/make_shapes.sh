#!/bin/bash

for i in $(seq 0 62); do
    echo "reflect.TypeOf((*struct {D [1 << $i]byte; P unsafe.Pointer})(nil)).Elem(),"
done
