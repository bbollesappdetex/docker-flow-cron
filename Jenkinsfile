pipeline {
  agent {
    label "test"
  }
  options {
    buildDiscarder(logRotator(numToKeepStr: '2'))
    disableConcurrentBuilds()
  }
  stages {
    stage("build") {
      steps {
        checkout scm
        sh "docker image build -t vfarcic/docker-flow-cron ."
        sh "docker tag vfarcic/docker-flow-cron vfarcic/docker-flow-cron:beta"
        withCredentials([usernamePassword(
          credentialsId: "docker",
          usernameVariable: "USER",
          passwordVariable: "PASS"
        )]) {
          sh "docker login -u $USER -p $PASS"
        }
        sh "docker push vfarcic/docker-flow-cron:beta"
        sh "docker image build -t vfarcic/docker-flow-cron-test -f Dockerfile.test ."
        sh "docker push vfarcic/docker-flow-cron-test"
        sh "docker image build -t vfarcic/docker-flow-cron-docs -f Dockerfile.docs ."
      }
    }
    stage("test") {
      environment {
        HOST_IP = "build.dockerflow.com"
        DOCKER_HUB_USER = "vfarcic"
      }
      steps {
        sh "docker-compose -f docker-compose-test.yml run --rm unit"
      }
    }
    stage("release") {
      when {
        branch "master"
      }
      steps {
        withCredentials([usernamePassword(
          credentialsId: "docker",
          usernameVariable: "USER",
          passwordVariable: "PASS"
        )]) {
          sh "docker login -u $USER -p $PASS"
        }
        sh "docker tag vfarcic/docker-flow-cron vfarcic/docker-flow-cron:0.${env.BUILD_NUMBER}"
        sh "docker push vfarcic/docker-flow-cron:0.${env.BUILD_NUMBER}"
        sh "docker push vfarcic/docker-flow-cron"
        sh "docker tag vfarcic/docker-flow-cron-docs vfarcic/docker-flow-cron-docs:0.${env.BUILD_NUMBER}"
        sh "docker push vfarcic/docker-flow-cron-docs:0.${env.BUILD_NUMBER}"
        sh "docker push vfarcic/docker-flow-cron-docs"
      }
    }
    stage("deploy") {
      when {
        branch "master"
      }
      agent {
        label "prod"
      }
      steps {
        sh "docker service update --image vfarcic/docker-flow-cron-docs:0.${env.BUILD_NUMBER} cron_docs"
      }
    }
  }
  post {
    always {
      sh "docker system prune -f"
    }
    failure {
      slackSend(
        color: "danger",
        message: "${env.JOB_NAME} failed: ${env.RUN_DISPLAY_URL}"
      )
    }
  }
}
