#include <sys/unistd.h>
#include <sys/fcntl.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <netdb.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <arpa/inet.h>

int main(int argc, char **argv) {
	int err;
	int sfd;
	int newfd;
	int n;
	char buf[256];
	struct addrinfo hints;
	struct addrinfo *result, *rp;
	struct sockaddr_in c_addr, s_addr; /* IPv4 */
	socklen_t c_len;
	struct sockaddr_in *res;
	socklen_t res_len;
	int *sndgetoptval, sndsetoptval, *rcvgetoptval, rcvsetoptval;
	socklen_t sndgetoptlen, sndsetoptlen, rcvgetoptlen, rcvsetoptlen;
	char host[] = "localhost";
	char serv[] = "9000";
	// char host[] = "google.com";
	// char serv[] = "http";

	memset(&hints, 0, sizeof(struct addrinfo));
	hints.ai_family = AF_INET;    /* Allow IPv4 */
	hints.ai_socktype = SOCK_STREAM; /* Stream socket */
	hints.ai_flags = AI_PASSIVE;    /* For wildcard IP address */
	hints.ai_protocol = 0;          /* Any protocol */
	hints.ai_canonname = NULL;
	hints.ai_addr = NULL;
	hints.ai_next = NULL;
	err = getaddrinfo(host, serv, &hints, &result);
	if (err != 0) {
		fprintf(stderr, "client: getaddrinfo: %d\n", err);
		return 1;
	}
	s_addr = *(struct sockaddr_in *)result->ai_addr;
	printf("Listening to %s:%d\n", inet_ntoa(s_addr.sin_addr), ntohs(s_addr.sin_port));

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

		res = malloc(sizeof(struct sockaddr_in));
		res_len = sizeof(struct sockaddr_in);
		getsockname(sfd, (struct sockaddr *)res, &res_len);
		printf("getsockname: after socket %s:%d\n", inet_ntoa(res->sin_addr), ntohs(res->sin_port));
	
		res = malloc(sizeof(struct sockaddr_in));
		res_len = sizeof(struct sockaddr_in);
		getpeername(sfd, (struct sockaddr *)res, &res_len);
		printf("getpeername: after socket %s:%d\n", inet_ntoa(res->sin_addr), ntohs(res->sin_port));

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

	res = malloc(sizeof(struct sockaddr_in));
	res_len = sizeof(struct sockaddr_in);
	getsockname(sfd, (struct sockaddr *)res, &res_len);
	printf("getsockname: after connect %s:%d\n", inet_ntoa(res->sin_addr), ntohs(res->sin_port));

	res = malloc(sizeof(struct sockaddr_in));
	res_len = sizeof(struct sockaddr_in);
	getpeername(sfd, (struct sockaddr *)res, &res_len);
	printf("getpeername: after connect %s:%d\n", inet_ntoa(res->sin_addr), ntohs(res->sin_port));

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
