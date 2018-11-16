package lib

import (
	"crypto/tls"
        "crypto/sha1"
        "errors"
        "io"
        "math/rand"
        "net"
        "time"
	"net/http"
)

const SEED = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrst0123456789"
const SEED_LEN = uint32(len(SEED))

func Sha1(input []byte) []byte {
        tmp := sha1.Sum(input)
        return tmp[:]
}

func ArrayCopy(src []byte, dest []byte) {

}
func ArrayEqual(data1, data2 []byte) bool {
        if len(data1) != len(data2) {
                return false
        }

        for idx, b1 := range (data1) {
                b2 := data2[idx]
                if b1 != b2 {
                        return false
                }
        }
        return true
}

func ArrayConcat(data1, data2 []byte) []byte {
        result := make([]byte, len(data1) + len(data2))
        for i:=0; i < len(data1); i++ {
                result[i] = data1[i]
        }

        for i:=0; i < len(data2); i++ {
                result[i+len(data1)] = data2[i]
        }
        return result
}

func ReadByte(r io.Reader) (byte, error) {
        buf := make([]byte, 1)
        _, err := r.Read(buf)
        if err != nil {
                return 0, err
        }
        return buf[0],nil
}

func WriteByte(w io.Writer, data byte) error {
        buf := make([]byte, 1)
        buf[0] = data
        _, err := w.Write(buf)

        if err != nil {
                return err
        }
        return nil
}

func RandomData(len int) []byte {
        data := make([]byte, len)
        for i:=0; i < len; i++ {
                data[i] = SEED[rand.Uint32() % SEED_LEN]
        }
        return data
}


// Write data up to 255 bytes
func WriteData(w *net.TCPConn, data []byte) error {
        w.SetWriteDeadline(time.Now().Add(10 * time.Second))
        defer func() {
                w.SetWriteDeadline(time.Time{})
        }()
        if len(data) > 255 {
                return errors.New("single write can't exceed 255 bytes")
        }
        buf := make([]byte, 1)
        buf[0] = byte(len(data))
        nwritten, err  := w.Write(buf)
        if err != nil {
                return err
        }
        if nwritten != 1 {
                return errors.New("insufficient write")
        }
        nwritten, err = w.Write(data)
        if err != nil {
                return err
        }
        if nwritten != len(data) {
                return errors.New("insufficient write")
        }
        return nil
}

func ReadData(r *net.TCPConn) ([]byte, error) {
        r.SetReadDeadline(time.Now().Add(10 * time.Second))
        defer func() {
                r.SetReadDeadline(time.Time{})
        }()
        buf := make([]byte, 1)
        nread, err := r.Read(buf)
        if err != nil {
                return []byte{}, err
        }

        if nread  != 1 {
                return []byte{}, errors.New("insufficient read")
        }

        strLen := buf[0]
        dataBuf := make([]byte, strLen)
        nread, err = r.Read(dataBuf)
        if err != nil {
                return []byte{}, err
        }

        if byte(nread) != strLen {
                return []byte{}, errors.New("insufficient read")
        }
        return dataBuf, nil
}


func JumperClient(remote string, secret string) *http.Client {
	j := &Jumper{remote, secret}
        tr := &http.Transport{
                Dial:                j.Dialer,
                TLSHandshakeTimeout: 2 * time.Second,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},

        }

        client := &http.Client{Transport: tr}
        return client
}

type Jumper struct {
	Remote string
	Secret string
}

func (v *Jumper) Dialer(netType string, connString string) (net.Conn, error) {
        if netType != "tcp" {
                return nil, errors.New("Only supports TCP")
        }
        tcpAddr, err := net.ResolveTCPAddr("tcp", v.Remote)
        if err != nil {
                return nil, err
        }
        conn, err := net.DialTCP("tcp", nil, tcpAddr)
        if err != nil {
                return nil, err
        }

        // read challenge
        challenge, err := ReadData(conn)
        if err != nil {
                conn.Close()
                return nil, err
        }

        tocalc := ArrayConcat(challenge, []byte(v.Secret))

        response := Sha1(tocalc)
        // Write challenge response
        err = WriteData(conn, response)
        if err != nil {
                conn.Close()
                return nil, err
        }

        // read challenge result
        status,err := ReadByte(conn)
        if err != nil {
                conn.Close()
                return nil, err
        }

        if status != 0 {
                // failed
                errorData,err := ReadData(conn)
                if err != nil {
                        conn.Close()
                        return nil, err
                } else {
                        conn.Close()
                        return nil, errors.New(string(errorData))
                }
        }

        // write what to connect to
        err = WriteData(conn, []byte(connString))
        if err != nil {
                conn.Close()
                return nil, err
        }

        status, err = ReadByte(conn)
        if err != nil {
                conn.Close()
                return nil, err
        }

        if status != 0 {
                // failed
                errorData,err := ReadData(conn)
                if err != nil {
                        conn.Close()
                        return nil, err
                } else {
                        conn.Close()
                        return nil, errors.New(string(errorData))
                }
        }
        return conn, nil
}

