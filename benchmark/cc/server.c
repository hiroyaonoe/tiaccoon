#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <sys/un.h>
#include <signal.h>

int sockfd;

void error(const char *msg) {
    perror(msg);
    exit(1);
}

void handle_sigint(int sig) {
    printf("Caught signal %d, closing socket and exiting...\n", sig);
    close(sockfd);
    exit(0);
}

void handle_client(int sock) {
    char buffer[256];
    int n;
    while ((n = read(sock, buffer, 255)) > 0) {
        buffer[n] = '\0';
        printf("Received: %s\n", buffer);
    }
    if (n < 0) error("ERROR reading from socket");
    close(sock);
}

int main(int argc, char *argv[]) {
    signal(SIGINT, handle_sigint);

    if (argc < 3) {
        fprintf(stderr, "Usage: %s protocol address\n", argv[0]);
        exit(1);
    }

    char *protocol = argv[1];
    char *address = argv[2];
    int newsockfd, portno;
    socklen_t clilen;
    struct sockaddr_in serv_addr, cli_addr;
    struct sockaddr_un serv_addr_un;

    if (strcmp(protocol, "TCP") == 0) {
        printf("Starting TCP server on port %s\n", address);
        sockfd = socket(AF_INET, SOCK_STREAM, 0);
        if (sockfd < 0) error("ERROR opening socket");

        memset((char *) &serv_addr, 0, sizeof(serv_addr));
        portno = atoi(address);
        serv_addr.sin_family = AF_INET;
        serv_addr.sin_addr.s_addr = INADDR_ANY;
        serv_addr.sin_port = htons(portno);

        if (bind(sockfd, (struct sockaddr *) &serv_addr, sizeof(serv_addr)) < 0)
            error("ERROR on binding");

        printf("TCP server listening on port %s\n", address);
        listen(sockfd, 5);
        clilen = sizeof(cli_addr);
        while (1) {
            newsockfd = accept(sockfd, (struct sockaddr *) &cli_addr, &clilen);
            if (newsockfd < 0) error("ERROR on accept");
            handle_client(newsockfd);
        }
    } else if (strcmp(protocol, "UNIX") == 0) {
        printf("Starting UNIX server on path %s\n", address);
        sockfd = socket(AF_UNIX, SOCK_STREAM, 0);
        if (sockfd < 0) error("ERROR opening socket");

        memset(&serv_addr_un, 0, sizeof(serv_addr_un));
        serv_addr_un.sun_family = AF_UNIX;
        strncpy(serv_addr_un.sun_path, address, sizeof(serv_addr_un.sun_path) - 1);

        unlink(address);
        if (bind(sockfd, (struct sockaddr *) &serv_addr_un, sizeof(serv_addr_un)) < 0)
            error("ERROR on binding");

        printf("UNIX server listening on path %s\n", address);
        listen(sockfd, 5);
        while (1) {
            newsockfd = accept(sockfd, NULL, NULL);
            if (newsockfd < 0) error("ERROR on accept");
            handle_client(newsockfd);
        }
    } else {
        fprintf(stderr, "Unsupported protocol: %s\n", protocol);
        exit(1);
    }

    close(sockfd);
    return 0;
}
