CC = gcc
CFLAGS = -Wall -O2
LDFLAGS = -lm

TARGETS = perf server
OBJS = $(TARGETS:=.o)

all: $(TARGETS)

$(TARGETS): %: %.o
	$(CC) $(CFLAGS) -o $@ $< $(LDFLAGS)

%.o: %.c
	$(CC) $(CFLAGS) -c $<

clean:
	rm -f $(OBJS) $(TARGETS)
