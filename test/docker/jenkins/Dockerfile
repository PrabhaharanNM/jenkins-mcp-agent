FROM jenkins/jenkins:lts

# Skip setup wizard
ENV JAVA_OPTS="-Djenkins.install.runSetupWizard=false"

# Install plugins
COPY plugins.txt /usr/share/jenkins/ref/plugins.txt
RUN jenkins-plugin-cli --plugin-file /usr/share/jenkins/ref/plugins.txt

# Create admin user via init script
COPY --chown=jenkins:jenkins init.groovy /usr/share/jenkins/ref/init.groovy.d/init.groovy

# Copy test pipeline
COPY Jenkinsfile.test /var/jenkins_home/Jenkinsfile.test
