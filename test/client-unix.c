#include <sys/unistd.h>
#include <sys/fcntl.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <netinet/in.h>
#include <netdb.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <arpa/inet.h>

int main(int argc, char **argv) {
	int err;
	int sfd;
	int n;
	char buf[256];
	struct addrinfo *result, *rp;
	struct sockaddr_un *c_addr, *s_addr; /* Unix */
	socklen_t c_len;
	struct sockaddr_un *res;
	socklen_t res_len;
	int *sndgetoptval, sndsetoptval, *rcvgetoptval, rcvsetoptval;
	socklen_t sndgetoptlen, sndsetoptlen, rcvgetoptlen, rcvsetoptlen;
	char host[] = "/tmp/test.sock";

	result = calloc(1,sizeof(struct addrinfo));
	result->ai_family = AF_UNIX;
	result->ai_addrlen = sizeof(struct sockaddr_un);
	result->ai_addr = malloc(result->ai_addrlen);
	result->ai_socktype = SOCK_STREAM;
	result->ai_protocol = 0;
	result->ai_next = NULL;

	s_addr = (struct sockaddr_un *) result->ai_addr;
	s_addr->sun_family = AF_UNIX;
	strncpy(s_addr->sun_path, host, sizeof(s_addr->sun_path));

	printf("Connecting to %s\n", host);

	for (rp = result; rp != NULL; rp = rp->ai_next) {
		sfd = socket(rp->ai_family, rp->ai_socktype,
			rp->ai_protocol);
		if (sfd == -1)
			continue;

		sndgetoptval = malloc(sizeof(int));
		rcvgetoptval = malloc(sizeof(int));
		sndgetoptlen = sizeof(int);
		rcvgetoptlen = sizeof(int);
		getsockopt(sfd, SOL_SOCKET, SO_SNDBUF, sndgetoptval, &sndgetoptlen);
		getsockopt(sfd, SOL_SOCKET, SO_RCVBUF, rcvgetoptval, &rcvgetoptlen);
		printf("getsockopt: after socket: snd %d rcv %d\n", *sndgetoptval, *rcvgetoptval);
		sndsetoptval = 262144;
		rcvsetoptval = 262144;
		sndsetoptlen = sizeof(int);
		rcvsetoptlen = sizeof(int);
		setsockopt(sfd, SOL_SOCKET, SO_SNDBUF, &sndsetoptval, sndsetoptlen);
		setsockopt(sfd, SOL_SOCKET, SO_RCVBUF, &rcvsetoptval, rcvsetoptlen);
		sndgetoptval = malloc(sizeof(int));
		rcvgetoptval = malloc(sizeof(int));
		sndgetoptlen = sizeof(int);
		rcvgetoptlen = sizeof(int);
		getsockopt(sfd, SOL_SOCKET, SO_SNDBUF, sndgetoptval, &sndgetoptlen);
		getsockopt(sfd, SOL_SOCKET, SO_RCVBUF, rcvgetoptval, &rcvgetoptlen);
		printf("getsockopt: after setsockopt: snd %d rcv %d\n", *sndgetoptval, *rcvgetoptval);

		res = malloc(sizeof(struct sockaddr_un));
		res_len = sizeof(struct sockaddr_un);
		getsockname(sfd, (struct sockaddr *)res, &res_len);
		printf("getsockname: after socket %s\n", res->sun_path);

		res = malloc(sizeof(struct sockaddr_un));
		res_len = sizeof(struct sockaddr_un);
		getpeername(sfd, (struct sockaddr *)res, &res_len);
		printf("getpeername: after socket %s\n", res->sun_path);

		if (connect(sfd, rp->ai_addr, rp->ai_addrlen) == 0)
			break; /* Success */
		close(sfd);
	}
	if (rp == NULL) { /* No address succeeded */
		fprintf(stderr, "Could not connect\n");
		return 1;
	}
	freeaddrinfo(result); /* No longer needed */

	printf("Connected\n");
	sndgetoptval = malloc(sizeof(int));
	rcvgetoptval = malloc(sizeof(int));
	sndgetoptlen = sizeof(int);
	rcvgetoptlen = sizeof(int);
	getsockopt(sfd, SOL_SOCKET, SO_SNDBUF, sndgetoptval, &sndgetoptlen);
	getsockopt(sfd, SOL_SOCKET, SO_RCVBUF, rcvgetoptval, &rcvgetoptlen);
	printf("getsockopt: after connect: snd %d rcv %d\n", *sndgetoptval, *rcvgetoptval);

	res = malloc(sizeof(struct sockaddr_un));
	res_len = sizeof(struct sockaddr_un);
	getsockname(sfd, (struct sockaddr *)res, &res_len);
	printf("getsockname: after connect %s\n", res->sun_path);

	res = malloc(sizeof(struct sockaddr_un));
	res_len = sizeof(struct sockaddr_un);
	getpeername(sfd, (struct sockaddr *)res, &res_len);
	printf("getpeername: after connect %s\n", res->sun_path);

	memset(buf, 0, 256);
	n = recv(sfd, buf, 255, 0);
	if (n < 0) {
		printf("failed to recv: %d", n);
		return 1;
	}
	printf("%s\n", buf);

	n = send(sfd, "ijklmnopq\n", sizeof("ijklmnopq\n"), 0);
	if (n < 0) {
		printf("failed to send: %d", n);
		return 1;
	}

	close(sfd);
}
