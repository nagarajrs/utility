CC=gcc
CFLAGS=-O3 -Wall -pthread
LDFLAGS=-lnuma

OBJS=main.o walker.o queue.o worker.o numa_util.o stats.o

fsx-warmd: $(OBJS)
	$(CC) $(CFLAGS) -o fsx-warmd $(OBJS) $(LDFLAGS)

clean:
	rm -f *.o fsx-warmd
