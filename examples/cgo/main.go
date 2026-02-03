package main

/*
#include <stdio.h>

void hello(const char *name) {
	printf("Hello, %s!\n", name);
	fflush(stdout);
}
*/
import "C"

func main() {
	C.hello(C.CString("DGD"))
}
