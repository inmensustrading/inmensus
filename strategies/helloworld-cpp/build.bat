@echo off
call "C:\Program Files (x86)\Microsoft Visual Studio\2017\Community\VC\Auxiliary\Build\vcvarsall.bat" x64     
set compilerflags = /Od /Zi /EHsc
set linkerflags = /OUT:helloworld.exe
cl.exe %compilerflags% helloworld.cpp /link %linkerflags%