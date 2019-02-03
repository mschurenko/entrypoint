package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"math"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

const testSecret = "/mschurenko/entrypoint/test_secret"
const testSecretValue = "mysecret"
const s3Bucket = "mschurenko-test"
const s3Key = "fixtures/vars.yml"

func TestMain(m *testing.M) {
	r := getRegion()
	sess := session.Must(session.NewSession(&aws.Config{Region: aws.String(r)}))
	setup(sess)
	rc := m.Run()
	teardown(sess)
	os.Exit(rc)
}

func describeSecret(svc *secretsmanager.SecretsManager) (*secretsmanager.DescribeSecretOutput, error) {
	input := &secretsmanager.DescribeSecretInput{SecretId: aws.String(testSecret)}
	return svc.DescribeSecret(input)
}

func secretExists(svc *secretsmanager.SecretsManager) bool {
	o, err := describeSecret(svc)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == secretsmanager.ErrCodeResourceNotFoundException {
				return false
			}
		}
		log.Fatalf("desribe secret failed: %v", err)
	}

	if o.DeletedDate == nil {
		return true
	}

	// secret exists but is still not deleted, so wait until secret is gone
	for i := 0; i < 10; i++ {
		if _, err := describeSecret(svc); err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				if awsErr.Code() == secretsmanager.ErrCodeResourceNotFoundException {
					break
				}
			}
		}
		time.Sleep(time.Duration(math.Exp2(float64(i))) * time.Second)
	}

	return false
}

func createSecret(svc *secretsmanager.SecretsManager) error {
	input := &secretsmanager.CreateSecretInput{
		Name:         aws.String(testSecret),
		SecretString: aws.String(testSecretValue),
	}
	_, err := svc.CreateSecret(input)
	return err
}

func deleteSecret(svc *secretsmanager.SecretsManager) error {
	input := &secretsmanager.DeleteSecretInput{
		ForceDeleteWithoutRecovery: aws.Bool(true),
		SecretId:                   aws.String(testSecret),
	}
	_, err := svc.DeleteSecret(input)
	return err
}

func updateSecret(svc *secretsmanager.SecretsManager) error {
	input := &secretsmanager.UpdateSecretInput{
		SecretId:     aws.String(testSecret),
		SecretString: aws.String(testSecretValue),
	}
	_, err := svc.UpdateSecret(input)
	return err
}

func putObject(svc *s3.S3, file string) error {
	bs, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	input := &s3.PutObjectInput{
		ContentType: aws.String("application/x-yaml "),
		Bucket:      aws.String(s3Bucket),
		Body:        bytes.NewReader(bs),
		Key:         aws.String(file),
	}

	_, err = svc.PutObject(input)

	return err
}

func setup(sess *session.Session) {
	// add yaml file to s3
	s3Svc := s3.New(sess)
	if err := putObject(s3Svc, s3Key); err != nil {
		log.Fatal(err)
	}

	// add secret
	smSvc := secretsmanager.New(sess)
	switch secretExists(smSvc) {
	case true:
		if err := updateSecret(smSvc); err != nil {
			log.Fatalf("update secret failed: %v", err)
		}
	default:
		if err := createSecret(smSvc); err != nil {
			log.Fatalf("create secret failed: %v", err)
		}
		for i := 0; i < 10; i++ {
			if !secretExists(smSvc) {
				time.Sleep(time.Duration(math.Exp2(float64(i))) * time.Second)
			} else {
				break
			}
		}
	}

}

func teardown(sess *session.Session) {
	svc := secretsmanager.New(sess)
	if err := deleteSecret(svc); err != nil {
		log.Fatalf("could not delete secret: %v\n", err)
	}
}

func TestCheckEntrypointVarValid(t *testing.T) {
	valid := "ENTRYPOINT_VARS_FILE"

	if !checkEntrypointVar(valid) {
		t.Errorf("%v should be a valid env var", valid)
	}

}

func TestCheckEntrypointVarInValid(t *testing.T) {
	invalid := "ENTRYPOINT_X"

	if checkEntrypointVar(invalid) {
		t.Errorf("%v should not be a valid env var", invalid)
	}

}

func TestRenderStr(t *testing.T) {
	tmpl := `{{ mul 2 2 }}`
	exepcted := `4`
	resp := renderStr("test", tmpl, nil)
	if resp != exepcted {
		t.Errorf("%v is not equal to %v\n", resp, exepcted)
	}

}

func TestGetVarsFromFileLocal(t *testing.T) {
	f := "./fixtures/vars.yml"
	getVarsFromFile(f)
}

func TestGetVarsFromFileS3(t *testing.T) {
	getVarsFromFile(s3Prefix + s3Bucket + "/" + s3Key)
}

func TestRenderTmpl(t *testing.T) {
	vars := getVarsFromFile(s3Prefix + s3Bucket + "/" + s3Key)
	ctx := tmplCtx{
		EnvVars: map[string]string{
			"MY_ENV": "production",
		},
		Vars: vars,
	}

	tmplName := "test.conf.tmpl"

	exceptedStr := `
	MY_ENV is production
	production web db is prod-db1
	production web cache is prod-cache1
	value of /mschurenko/entrypoint/test_secret is mysecret
	aws region is us-west-2
	value of production.web.password is mysecret
	`

	tmplStr := `
	MY_ENV is {{ .EnvVars.MY_ENV }}
	production web db is {{ (index .Vars .EnvVars.MY_ENV).web.db }}
	production web cache is {{ (index .Vars .EnvVars.MY_ENV).web.cache }}
	value of /mschurenko/entrypoint/test_secret is {{ getSecret "/mschurenko/entrypoint/test_secret" }}
	aws region is {{ getRegion }}
	value of production.web.password is {{ .Vars.production.web.password }}
	`

	if err := ioutil.WriteFile(tmplName, []byte(tmplStr), 0644); err != nil {
		t.Error(err)
	}

	defer os.Remove(tmplName)

	renderTmpl(tmplName, ctx)

	defer os.Remove(stripExt(tmplName))

	sb, err := ioutil.ReadFile(stripExt(tmplName))
	if err != nil {
		t.Error(err)
	}

	if string(sb) != exceptedStr {
		t.Errorf("%v is not equal to %v", string(sb), exceptedStr)
	}

}
